// Package server composes ingestion, recording, state, analysis, scheduling and HTTP.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/analytics"
	"f1tele/server/internal/auth"
	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/engineer"
	"f1tele/server/internal/history"
	"f1tele/server/internal/httpapi"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/setup"
	"f1tele/server/internal/state"
	"f1tele/server/internal/strategy"
	"f1tele/server/internal/voice"
	"f1tele/server/internal/web"
)

const Version = "1.0.0"

type Server struct {
	cfg      *config.Store
	catalog  *history.Catalog
	recorder *history.Recorder
	reducer  *state.Reducer
	engine   *analysis.Engine
	eval     *strategy.Evaluator
	arbiter  *strategy.Arbiter
	http     *http.Server
	api      *httpapi.API
	seq      *seqAlloc
	cancel   context.CancelFunc
	once     sync.Once

	engineerMgr                           *engineer.Manager
	stats                                 *IntakeStats
	lastContextFP                         string
	lastSession, lastFragment, lastBranch string
	closedSession                         string
	closedFragment                        string
	eventIndex                            int
	eventSession                          string
	laps                                  map[int]lapState
	tracker                               *analytics.Tracker
}

type lapState struct {
	lap      int
	startSeq uint64
	invalid  bool
	pit      int
	compound int
}

func New(store *config.Store, logger *slog.Logger) (*Server, error) {
	cfg := store.Config()
	dir := store.DataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	cat, err := history.OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		return nil, err
	}
	if err := history.ReconcileRecordings(context.Background(), cat); err != nil {
		return nil, err
	}
	if err := cat.CloseStaleSessions(context.Background(), time.Now().UnixMilli()); err != nil {
		return nil, err
	}
	rec := history.NewRecorder(cfg.Recording, cat, dir, nil)
	src := "ctx-" + installID(dir)
	if err := cat.EnsureSourceContext(context.Background(), src, "local"); err != nil {
		return nil, err
	}
	reducer := state.New(src, time.Now)
	rec.SetGapSink(reducer.SetRecordingGap)
	reducer.SetSessionLookup(func(sourceCtx string, season, weekend, session uint64) (string, bool) {
		id, lookupErr := cat.LinkedSession(context.Background(), sourceCtx, season, weekend, session)
		return id, lookupErr == nil && id != ""
	})
	s := &Server{cfg: store, catalog: cat, recorder: rec, reducer: reducer, laps: map[int]lapState{}, stats: &IntakeStats{}}
	s.engine = &analysis.Engine{Catalog: cat, DataDir: dir}
	s.tracker = analytics.NewTracker(cat)
	s.eval = strategy.NewEvaluator(s.engine, cfg)
	s.arbiter = strategy.NewArbiter(cfg.Notifications, cfg.ReferenceProfiles, cfg.CapacityProfiles)
	s.seq = &seqAlloc{cat: cat, ctx: context.Background(), id: src}
	setupSvc := setup.New(store, cat)
	authStore := auth.New(store.DataDir())
	if _, err := authStore.Ensure(); err != nil {
		return nil, err
	}
	inv := &engineer.Invoker{Backend: engineer.ToolBackend{Engine: s.engine, Eval: s.eval, Snapshot: func() state.Snapshot { return reducer.Snapshot(nil, nil) },
		Search: func(ctx context.Context, q map[string]any) (contract.SearchResult, error) {
			return setupSvc.Search(ctx, httpapi.SetupQuery{Query: fmt.Sprint(q["query"]), Conditions: fmt.Sprint(q["conditions"]), Assists: fmt.Sprint(q["assists"]), UseCase: fmt.Sprint(q["use_case"]), MaxResults: 5})
		},
		Fetch: setupSvc.Fetch,
	}}
	mgr := engineer.NewManager(store, reducer, cat, inv, s.arbiter)
	voiceSvc := voice.New(store)
	voiceSvc.AcceptedText = mgr.AcceptedText
	s.engineerMgr = mgr
	s.api = httpapi.New(httpapi.Deps{
		Config: store, Auth: authStore, Catalog: cat, Reducer: reducer, Recorder: rec, Engine: s.engine,
		Evaluator: s.eval, Arbiter: s.arbiter, Engineer: mgr, Setup: setupSvc, Voice: voiceSvc, InstanceID: "inst-" + installID(dir),
		Version: Version, StartedAt: time.Now(), Hub: httpapi.NewHub(),
		OnRecordStart: func() error { return rec.Resume() },
		OnRecordStop:  func() { rec.StopByUser(); reducer.SetRecordingState(history.RecStoppedUser) },
		Stop:          func() { go s.Shutdown(10 * time.Second) },
	})
	mgr.Activity = &engineer.Activity{Catalog: cat, Broadcast: s.api.AgentActivity.Publish}
	mgr.Activity.Reconcile(context.Background())
	go analytics.Backfill(context.Background(), cat, slog.Default(), 8)
	return s, nil
}

func installID(dir string) string {
	path := filepath.Join(dir, "install.id")
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return string(raw)
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	id := hex.EncodeToString(b)
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}

func (s *Server) Catalog() *history.Catalog   { return s.catalog }
func (s *Server) Recorder() *history.Recorder { return s.recorder }
func (s *Server) Reducer() *state.Reducer     { return s.reducer }
func (s *Server) API() *httpapi.API           { return s.api }
func (s *Server) Intake() *IntakeStats        { return s.stats }

func (s *Server) Run(ctx context.Context, logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	cfg := s.cfg.Config()
	s.recorder.Start()
	udp, err := net.ListenPacket("udp", net.JoinHostPort(cfg.UDP.Host, strconv.Itoa(cfg.UDP.Port)))
	if err != nil {
		return fmt.Errorf("bind udp %s:%d: %w", cfg.UDP.Host, cfg.UDP.Port, err)
	}
	defer udp.Close()
	go s.ingest(ctx, udp, logger)
	go s.loop(ctx, logger)
	addr := net.JoinHostPort(cfg.HTTP.Host, strconv.Itoa(cfg.HTTP.Port))
	s.http = &http.Server{Addr: addr, Handler: s.api.Handler(web.FS()), ReadHeaderTimeout: 5 * time.Second}
	logger.Info("f1tele listening", "http", addr, "udp", net.JoinHostPort(cfg.UDP.Host, strconv.Itoa(cfg.UDP.Port)), "data", s.cfg.DataDir())
	errCh := make(chan error, 1)
	go func() { errCh <- s.http.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, sc := context.WithTimeout(context.Background(), 10*time.Second)
	defer sc()
	_ = s.http.Shutdown(shutdownCtx)
	return nil
}

func (s *Server) Shutdown(timeout time.Duration) {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.reducer != nil && s.catalog != nil {
			cx := s.reducer.Context()
			now := time.Now().UnixMilli()
			_ = s.catalog.CloseFragment(context.Background(), cx.FragmentID, now)
			_ = s.catalog.CloseSession(context.Background(), cx.SessionID, now)
		}
		if s.tracker != nil {
			_ = s.tracker.Close(context.Background())
		}
		done := make(chan struct{})
		go func() { s.recorder.Stop(); close(done) }()
		select {
		case <-done:
		case <-time.After(timeout):
		}
		if s.http != nil {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			_ = s.http.Shutdown(ctx)
		}
		s.catalog.Close()
	})
}

func (s *Server) ingest(ctx context.Context, conn net.PacketConn, logger *slog.Logger) {
	buf := make([]byte, 65535)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("udp read", "err", err)
			continue
		}
		at := time.Now()
		data := append([]byte(nil), buf[:n]...)
		seq, err := s.seq.Next()
		if err != nil {
			logger.Error("sequence allocation failed", "err", err)
			continue
		}
		p, derr := protocol.Decode(data)
		reason := ""
		if derr != nil {
			reason = derr.Error()
		}
		s.stats.Observe(n, data)
		s.reducer.Apply(seq, at, data, p, reason)
		if derr == nil {
			cx := s.reducer.Context()
			length, _ := s.reducer.Value("session.track_length_m", 0)
			s.tracker.Observe(ctx, seq, p, cx, map[string]float64{"track_length_m": length})
		}
		if s.reducer.SessionID() != "" {
			cx := s.reducer.Context()
			s.recorder.SetIdentity(cx.SessionID, cx.FragmentID, cx.SourceContextID)
			if cx.SessionID != s.lastSession || cx.FragmentID != s.lastFragment {
				// Catalog rows must exist before the writer creates its FK row.
				s.persistIdentityAndLaps(ctx, p, seq)
			}
		}
		s.recorder.Record(seq, at, data)
		if reason != "" {
			continue
		}
		if pid := p.Header.PacketID; pid == 1 || pid == 2 || pid == 4 || pid == 16 {
			s.persistIdentityAndLaps(ctx, p, seq)
		}
	}
}

func (s *Server) loop(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	alertTicker := time.NewTicker(100 * time.Millisecond)
	defer alertTicker.Stop()
	closeCheck := time.NewTicker(time.Second)
	defer closeCheck.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.persistEvents()
		case <-alertTicker.C:
			car := s.reducer.PlayerCar()
			snap := s.reducer.Snapshot([]int{car}, liveChannels)
			s.eval.Observe(snap)
			s.engineerMgr.Tick(snap)
			notes := s.arbiter.Observe(snap)
			for _, n := range notes {
				logger.Info("driver notification", "alert", n.AlertID, "text", n.Text, "critical", n.Critical, "spoken", n.Spoken)
			}
			recState, _ := s.recorder.State()
			s.reducer.SetRecordingState(recState)
		case <-closeCheck.C:
			// A quiet session is closed once; a new logical session also closes the
			// previous one in the tracker. The window is configurable and read live.
			inactivity := time.Duration(s.cfg.Config().Session.InactivitySeconds) * time.Second
			if last := s.reducer.LastPacketAt(); !last.IsZero() && time.Since(last) > inactivity {
				cx := s.reducer.Context()
				now := time.Now().UnixMilli()
				if cx.SessionID != "" && cx.SessionID != s.closedSession {
					if err := s.catalog.CloseSession(ctx, cx.SessionID, now); err != nil {
						logger.Warn("close session", "err", err)
					}
					s.closedSession = cx.SessionID
				}
				if cx.FragmentID != "" && cx.FragmentID != s.closedFragment {
					if err := s.catalog.CloseFragment(ctx, cx.FragmentID, now); err != nil {
						logger.Warn("close fragment", "err", err)
					}
					s.closedFragment = cx.FragmentID
				}
				_ = s.tracker.Close(ctx)
			}
		}
	}
}

func (s *Server) persistIdentityAndLaps(ctx context.Context, p *protocol.Packet, seq uint64) {
	cx := s.reducer.Context()
	if cx.SessionID != "" && cx.SessionID == s.closedSession {
		_ = s.catalog.ReopenSession(ctx, cx.SessionID)
		s.closedSession = ""
	}
	if cx.FragmentID != "" && cx.FragmentID == s.closedFragment {
		_ = s.catalog.ReopenFragment(ctx, cx.FragmentID)
		s.closedFragment = ""
	}
	if cx.SessionID != s.lastSession {
		if s.lastSession != "" {
			_ = s.catalog.CloseSession(ctx, s.lastSession, time.Now().UnixMilli())
		}
		_ = s.catalog.CreateSession(ctx, cx.SessionID, cx)
		// A resumed session keeps its original start time and becomes live again.
		_ = s.catalog.ReopenSession(ctx, cx.SessionID)
		s.lastSession = cx.SessionID
		s.closedSession = ""
	}
	if p.Header.PacketID == 1 || p.Header.PacketID == 16 {
		fp := ctxFingerprint(cx)
		if fp != s.lastContextFP {
			_ = s.catalog.UpdateSessionContext(ctx, cx.SessionID, cx)
			s.lastContextFP = fp
		}
	}
	if cx.FragmentID != s.lastFragment {
		if s.lastFragment != "" {
			_ = s.catalog.CloseFragment(ctx, s.lastFragment, time.Now().UnixMilli())
		}
		f := contract.Fragment{ID: cx.FragmentID, SessionID: cx.SessionID, SourceContextID: cx.SourceContextID, GameSessionUID: cx.GameSessionUID, StartedAtMs: time.Now().UnixMilli(), Range: contract.Range{FromSeq: strconv.FormatUint(seq, 10), ToSeq: strconv.FormatUint(seq, 10)}}
		_ = s.catalog.CreateFragment(ctx, f)
		// A fragment closed by inactivity or startup reconciliation becomes live again.
		_ = s.catalog.ReopenFragment(ctx, cx.FragmentID)
		s.lastFragment = cx.FragmentID
	}
	if cx.BranchID != s.lastBranch {
		b := contract.Branch{ID: cx.BranchID, SessionID: cx.SessionID, IsSurviving: true}
		for _, br := range s.reducer.LastBranch() {
			if br.ID == cx.BranchID {
				b = br
			}
		}
		_ = s.catalog.CreateBranch(ctx, b)
		_ = s.catalog.SetSurviving(ctx, cx.SessionID, cx.BranchID)
		s.lastBranch = cx.BranchID
	}
	if p.Header.PacketID == 7 {
		for car := 0; car < p.CarCount(); car++ {
			if v, ok := p.CarField("m_actualTyreCompound", car); ok {
				st := s.laps[car]
				st.compound = int(v.F)
				s.laps[car] = st
			}
		}
	}
	if p.Header.PacketID != 2 {
		return
	}
	neutralized := false
	if snap := s.reducer.Snapshot(nil, []string{"session.safety_car_status"}); snap.Session != nil {
		neutralized = snap.Session["safety_car_status"] > 0
	}
	for car := 0; car < p.CarCount(); car++ {
		lapV, ok := p.CarField("m_currentLapNum", car)
		if !ok || lapV.F <= 0 {
			continue
		}
		lap := int(lapV.F)
		st := s.laps[car]
		invalid, _ := p.CarField("m_currentLapInvalid", car)
		pit, _ := p.CarField("m_pitStatus", car)
		if st.lap != 0 && lap == st.lap+1 {
			timeMs, _ := p.CarField("m_lastLapTimeInMS", car)
			l := contract.LapSummary{
				ID: fmt.Sprintf("lap-%s-%d-%d", cx.BranchID, car, st.lap), SessionID: cx.SessionID, BranchID: cx.BranchID,
				CarIndex: car, Lap: st.lap, StartSeq: strconv.FormatUint(st.startSeq, 10), EndSeq: strconv.FormatUint(seq-1, 10),
				LapTimeMs: contract.Available(int64(timeMs.F), "ms", "udp.packet2"), DistanceM: contract.Unavailable("not_received", "m", "catalog.laps", "distance is not catalogued"),
				Compound: contract.Available(compoundName(st.compound), "compound", "udp.packet7"),
				Valid:    !st.invalid, PitLap: st.pit != 0, Neutralized: neutralized, Complete: true, Gaps: []string{}, Conditions: conditions(cx),
			}
			_ = s.catalog.InsertLap(ctx, l)
			st.startSeq = seq
		} else if st.lap != 0 && lap != st.lap {
			st.startSeq = seq
		}
		st.lap = lap
		st.invalid = invalid.F != 0
		st.pit = int(pit.F)
		s.laps[car] = st
	}
}

var liveChannels = []string{
	"tyre.fl.wear_pct", "tyre.fr.wear_pct", "tyre.rl.wear_pct", "tyre.rr.wear_pct",
	"tyre.fl.surface_c", "tyre.fr.surface_c", "tyre.rl.surface_c", "tyre.rr.surface_c",
	"fuel.finish_margin_laps", "energy.store_j", "tyre.compound",
}

// ctxFingerprint avoids a catalog transaction per session packet; only real
// evidence changes are persisted.
func ctxFingerprint(cx contract.Context) string {
	return fmt.Sprintf("%v|%v|%v|%v|%v|%v|%v|%v|%v|%v", cx.TrackID, cx.RegulationSeason, cx.ModeFamily, cx.GameModeCode, cx.SessionTypeCode, cx.Online, cx.FormulaCode, cx.SeasonLinkID, cx.WeekendLinkID, cx.SessionLinkID)
}

func conditions(cx contract.Context) string {
	if cx.TrackID != nil && cx.RegulationSeason != nil {
		return fmt.Sprintf("track=%d;season=%d", *cx.TrackID, *cx.RegulationSeason)
	}
	return ""
}

func compoundName(v int) string { return protocol.CompoundLabel(v) }

func (s *Server) persistEvents() {
	cx := s.reducer.Context()
	events := s.reducer.Events()
	if cx.SessionID != s.eventSession {
		s.eventIndex = 0
		s.eventSession = cx.SessionID
	}
	if s.eventIndex > len(events) {
		s.eventIndex = 0
	}
	for ; s.eventIndex < len(events); s.eventIndex++ {
		_ = s.catalog.InsertEvent(context.Background(), cx.SessionID, cx.BranchID, events[s.eventIndex])
	}
}

type seqAlloc struct {
	mu   sync.Mutex
	cat  *history.Catalog
	ctx  context.Context
	id   string
	next uint64
	end  uint64
}

func (a *seqAlloc) Next() (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.next == 0 || a.next > a.end {
		start, err := a.cat.NextSeq(a.ctx, a.id)
		if err != nil {
			return 0, err
		}
		a.next = start
		a.end = start + 4095
	}
	v := a.next
	a.next++
	return v, nil
}
