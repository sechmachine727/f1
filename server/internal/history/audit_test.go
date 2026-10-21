package history

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/state"
)

func TestCloseSessionAndFragmentAreIdempotent(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []int64{5000, 4000} {
		if err := cat.CloseSession(ctx, "s", at); err != nil {
			t.Fatal(err)
		}
		if err := cat.CloseFragment(ctx, "f", at); err != nil {
			t.Fatal(err)
		}
	}
	var sessEnd, fragEnd int64
	if err := cat.DB().QueryRow("SELECT ended_at_ms FROM sessions WHERE id='s'").Scan(&sessEnd); err != nil {
		t.Fatal(err)
	}
	if err := cat.DB().QueryRow("SELECT ended_at_ms FROM fragments WHERE id='f'").Scan(&fragEnd); err != nil {
		t.Fatal(err)
	}
	if sessEnd != 5000 || fragEnd != 5000 {
		t.Fatalf("ended at %d/%d", sessEnd, fragEnd)
	}
	if err := cat.ReopenSession(ctx, "s"); err != nil {
		t.Fatal(err)
	}
	if err := cat.ReopenFragment(ctx, "f"); err != nil {
		t.Fatal(err)
	}
	var reopened int
	if err := cat.DB().QueryRow("SELECT ended_at_ms IS NULL FROM sessions WHERE id='s'").Scan(&reopened); err != nil || reopened != 1 {
		t.Fatalf("session reopen = %d err %v", reopened, err)
	}
	if err := cat.DB().QueryRow("SELECT ended_at_ms IS NULL FROM fragments WHERE id='f'").Scan(&reopened); err != nil || reopened != 1 {
		t.Fatalf("fragment reopen = %d err %v", reopened, err)
	}
}

func TestLapsFollowBranchLineage(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	parent := "b"
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateBranch(ctx, contract.Branch{ID: "b2", SessionID: "s", ParentID: &parent, IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	if err := cat.SetSurviving(ctx, "s", "b2"); err != nil {
		t.Fatal(err)
	}
	for _, l := range []contract.LapSummary{
		{ID: "l1", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, StartSeq: "1", EndSeq: "2", LapTimeMs: contract.Available(90000, "ms", "t"), Valid: true, Complete: true, Gaps: []string{}},
		{ID: "l2", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 2, StartSeq: "3", EndSeq: "4", LapTimeMs: contract.Available(91000, "ms", "t"), Valid: true, Complete: true, Gaps: []string{}},
		{ID: "l2b", SessionID: "s", BranchID: "b2", CarIndex: 0, Lap: 2, StartSeq: "5", EndSeq: "6", LapTimeMs: contract.Available(90500, "ms", "t"), Valid: true, Complete: true, Gaps: []string{}},
		{ID: "l3", SessionID: "s", BranchID: "b2", CarIndex: 0, Lap: 3, StartSeq: "7", EndSeq: "8", LapTimeMs: contract.Available(92000, "ms", "t"), Valid: true, Complete: true, Gaps: []string{}},
	} {
		if err := cat.InsertLap(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	got, err := cat.Laps(ctx, "s", "b2", nil)
	if err != nil || len(got) != 3 {
		t.Fatalf("lineage laps %d err %v", len(got), err)
	}
	if got[1].Lap != 2 || got[1].ID != "l2b" {
		t.Fatalf("rewound lap did not come from the child branch: %#v", got[1])
	}
}

func TestCreateSessionKeepsExistingRow(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	var started int64
	if err := cat.DB().QueryRow("SELECT started_at_ms FROM sessions WHERE id='s'").Scan(&started); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "race"}); err != nil {
		t.Fatal(err)
	}
	var again int64
	if err := cat.DB().QueryRow("SELECT started_at_ms FROM sessions WHERE id='s'").Scan(&again); err != nil {
		t.Fatal(err)
	}
	if again != started {
		t.Fatalf("session start replaced: %d -> %d", started, again)
	}
}

func TestCloseStaleSessionsClosesRecentOpenRows(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CloseStaleSessions(ctx, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	var closed int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM sessions WHERE id='s' AND ended_at_ms IS NOT NULL").Scan(&closed); err != nil || closed != 1 {
		t.Fatalf("session closed = %d err %v", closed, err)
	}
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM fragments WHERE id='f' AND ended_at_ms IS NOT NULL").Scan(&closed); err != nil || closed != 1 {
		t.Fatalf("fragment closed = %d err %v", closed, err)
	}
}

func TestRecordingFailureClosesRowAndResumesOnce(t *testing.T) {
	cat, dir := newTestCatalog(t)
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := defaultRecCfg()
	rec := NewRecorder(cfg, cat, dir, nil)
	rec.Start()
	rec.SetIdentity("s", "f", "c")
	pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: 1}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
	rec.Record(1, time.Now(), pkt)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := cat.DB().QueryRow("SELECT COUNT(*) FROM recordings WHERE fragment_id='f'").Scan(&n); err == nil && n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Stop the loop before injecting the failure so the test does not race the
	// writer; production failures are raised from inside the loop.
	rec.Stop()
	rec.fail(errors.New("disk failure"), 1)
	var state string
	if err := cat.DB().QueryRow("SELECT state FROM recordings WHERE fragment_id='f'").Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != RecStoppedError {
		t.Fatalf("recording state = %s", state)
	}
	if err := rec.Resume(); err != nil {
		t.Fatalf("resume after failure: %v", err)
	}
	rec.Stop()
}

func TestTheoreticalBestUsesIndependentSectorOwners(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	for _, l := range []LapAnalytics{
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, LapMs: 70000, Sector1Ms: 10000, Sector2Ms: 30000, Sector3Ms: 30000, Valid: true, Complete: true},
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 2, LapMs: 70000, Sector1Ms: 11000, Sector2Ms: 29000, Sector3Ms: 30000, Valid: true, Complete: true},
	} {
		if err := cat.UpsertLapAnalytics(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if err := cat.UpsertSessionAnalytics(ctx, "s", "b", "test"); err != nil {
		t.Fatal(err)
	}
	_, theo, _, _, err := cat.SessionAnalytics(ctx, "s")
	if err != nil {
		t.Fatal(err)
	}
	if theo != 69000 {
		t.Fatalf("theoretical best = %d, want 69000", theo)
	}
}

func seedRecording(t *testing.T, cat *Catalog, dir, sessionID, fragmentID, sourceCtx string, packets int, cfg contract.RecordingConfig) []string {
	t.Helper()
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, sourceCtx, "ip")
	_ = cat.CreateSession(ctx, sessionID, contract.Context{SourceContextID: sourceCtx, ModeFamily: "solo"})
	_ = cat.CreateFragment(ctx, contract.Fragment{ID: fragmentID, SessionID: sessionID, SourceContextID: sourceCtx, StartedAtMs: 1})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: fragmentID + "-br", SessionID: sessionID, IsSurviving: true})
	rec := NewRecorder(cfg, cat, dir, nil)
	rec.Start()
	rec.SetIdentity(sessionID, fragmentID, sourceCtx)
	for i := 0; i < packets; i++ {
		pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: float64(i) * 0.5}, nil,
			map[int]map[string]float64{0: {"m_speed": 200}}, nil)
		rec.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)*10), pkt)
	}
	rec.Stop()
	paths, err := cat.RecordingsForSession(ctx, sessionID)
	if err != nil || len(paths) == 0 {
		t.Fatalf("recordings %v err %v", paths, err)
	}
	return paths
}

func TestReconcileReadableRecordingClosesRowAndRecordsGap(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	seedRecording(t, cat, dir, "s", "f", "c", 40, cfg)
	ctx := context.Background()
	if _, err := cat.DB().ExecContext(ctx, "UPDATE recordings SET state='active', last_seq=999"); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRecordings(ctx, cat); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := cat.DB().QueryRowContext(ctx, "SELECT state FROM recordings WHERE fragment_id='f'").Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "complete" {
		t.Fatalf("recording state = %s", state)
	}
	var reason string
	var from, to int64
	if err := cat.DB().QueryRowContext(ctx, "SELECT from_seq, to_seq, reason FROM gaps WHERE session_id='s'").Scan(&from, &to, &reason); err != nil {
		t.Fatal(err)
	}
	if from != 41 || to != 999 || !strings.Contains(reason, "tail was lost") {
		t.Fatalf("gap %d-%d %s", from, to, reason)
	}
}

func TestReconcileDamagedRecordingKeepsSourceAndAddsRecovered(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	cfg.ChunkBytes = 4096
	paths := seedRecording(t, cat, dir, "s", "f", "c", 400, cfg)
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(dir, "damaged.mcap")
	if err := os.WriteFile(damaged, raw[:len(raw)-2000], 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := cat.CreateRecording(ctx, contract.Recording{ID: "damaged", FragmentID: "f", State: "active",
		Range: contract.Range{FromSeq: "1", ToSeq: "400"}}, damaged, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileRecordings(ctx, cat); err != nil {
		t.Fatal(err)
	}
	var failed, recovered int
	_ = cat.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM recordings WHERE id='damaged' AND state='failed'").Scan(&failed)
	_ = cat.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM recordings WHERE state='recovered'").Scan(&recovered)
	if failed != 1 || recovered != 1 {
		t.Fatalf("failed=%d recovered=%d", failed, recovered)
	}
	if _, err := os.Stat(damaged); err != nil {
		t.Fatal("source recording was removed")
	}
	var gaps int
	_ = cat.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM gaps WHERE session_id='s'").Scan(&gaps)
	if gaps == 0 {
		t.Fatal("recovered recording recorded no gap")
	}
}

func TestReplayStreamsRotatedFilesInOrder(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	cfg.RotateBytes = 8 << 10
	cfg.ChunkBytes = 1 << 10
	paths := seedRecording(t, cat, dir, "s", "f", "c", 400, cfg)
	if len(paths) < 2 {
		t.Fatalf("rotated files = %d", len(paths))
	}
	src, err := LoadReplaySession(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	rp := NewReplay("r", "s", "b", 1, false, src)
	rp.Step(400)
	if got := rp.View().PlayheadSessionTimeMs; got != 199500 {
		t.Fatalf("playhead after rotated replay = %d, want 199500", got)
	}
	rp.Close()
}

func TestReplaySelectsRequestedBranchSegments(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	paths := seedRecording(t, cat, dir, "s", "f", "c", 20, cfg)
	segs := []Segment{{BranchID: "alt", FromSeq: 11, ToSeq: 15}}
	src, err := LoadReplaySession(paths, segs)
	if err != nil {
		t.Fatal(err)
	}
	rp := NewReplay("r", "s", "alt", 1, false, src)
	rp.Step(100)
	if got := rp.View().PlayheadSessionTimeMs; got != 7000 {
		t.Fatalf("branch replay playhead = %d, want 7000", got)
	}
	rp.Close()
}

func TestSavedRaceResumeJoinsLinkedSession(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "ctx", "ip")
	link := func(v string) *string { return &v }
	if err := cat.CreateSession(ctx, "s", contract.Context{
		SourceContextID: "ctx", ModeFamily: "solo",
		SeasonLinkID: link("1"), WeekendLinkID: link("2"), SessionLinkID: link("3"),
	}); err != nil {
		t.Fatal(err)
	}
	r := state.New("ctx", time.Now)
	r.SetSessionLookup(func(src string, season, weekend, session uint64) (string, bool) {
		id, err := cat.LinkedSession(context.Background(), src, season, weekend, session)
		return id, err == nil && id != ""
	})
	var seq uint64
	feed := func(uid uint64, sessionLink float64) {
		data := protocol.BuildPacket(2026, 1, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: uid}, map[string]float64{
			"m_seasonLinkIdentifier": 1, "m_weekendLinkIdentifier": 2, "m_sessionLinkIdentifier": sessionLink,
		}, nil, nil)
		p, err := protocol.Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		seq++
		r.Apply(seq, time.Now(), data, p, "")
	}
	feed(111, 3)
	if got := r.Context().SessionID; got != "s" {
		t.Fatalf("saved race did not join the existing session: %s", got)
	}
	first := r.Context().FragmentID
	feed(222, 3)
	if got := r.Context().SessionID; got != "s" {
		t.Fatalf("resumed race created session %s", got)
	}
	if second := r.Context().FragmentID; second == first {
		t.Fatal("resumed race did not open a new fragment")
	}
	feed(333, 4)
	if got := r.Context().SessionID; got == "s" {
		t.Fatal("a genuinely new session joined the old one")
	}
}

func TestUnassignedSeasonFilter(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	season := func(v int) *int { return &v }
	for _, s := range []struct {
		id     string
		season *int
	}{{"a", season(2025)}, {"b", season(2026)}, {"c", nil}} {
		if err := cat.CreateSession(ctx, s.id, contract.Context{SourceContextID: "c", ModeFamily: "solo", RegulationSeason: s.season}); err != nil {
			t.Fatal(err)
		}
	}
	items, _, err := cat.ListSessions(ctx, SessionFilter{SeasonUnassigned: true})
	if err != nil || len(items) != 1 || items[0].ID != "c" {
		t.Fatalf("unassigned filter %#v err %v", items, err)
	}
	zero := 0
	items, _, err = cat.ListSessions(ctx, SessionFilter{Season: &zero})
	if err != nil || len(items) != 1 || items[0].ID != "c" {
		t.Fatalf("season=0 filter %#v err %v", items, err)
	}
}

func TestAgentLogDetailExposesLegacyOutputJSON(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	if _, err := cat.DB().ExecContext(ctx, `INSERT INTO invocations(id, session_id, branch_id, status, profile, output_json, started_at_ms)
		VALUES('i1','s','b','accepted','post_race','{"decision":"hold position"}',1)`); err != nil {
		t.Fatal(err)
	}
	rows, _, err := cat.ListAgentLogs(ctx, AgentLogFilter{InvocationID: "i1", Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("detail rows %#v err %v", rows, err)
	}
	if !json.Valid(rows[0].OutputJSON) || !strings.Contains(string(rows[0].OutputJSON), "hold position") {
		t.Fatalf("stored output missing: %q", rows[0].OutputJSON)
	}
	raw, _ := json.Marshal(rows[0])
	if !strings.Contains(string(raw), `"output_json"`) {
		t.Fatalf("response field missing: %s", raw)
	}
	list, _, err := cat.ListAgentLogs(ctx, AgentLogFilter{Limit: 1})
	if err != nil || len(list) != 1 {
		t.Fatalf("list rows %#v err %v", list, err)
	}
	if len(list[0].OutputJSON) != 0 {
		t.Fatal("list responses should not carry the full stored output")
	}
}

func TestRecorderStopRecordsGapAndReportsIt(t *testing.T) {
	cat, dir := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo"})
	_ = cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	cfg, _ := defaultRecCfg()
	cfg.QueueBytes = 1
	rec := NewRecorder(cfg, cat, dir, nil)
	var reported string
	rec.SetGapSink(func(reason string) { reported = reason })
	rec.Start()
	rec.SetIdentity("s", "f", "c")
	rec.Record(10, time.Now(), make([]byte, 100))
	if state, _ := rec.State(); state != RecStoppedLimit {
		t.Fatalf("recorder state = %s", state)
	}
	var reason string
	if err := cat.DB().QueryRowContext(ctx, "SELECT reason FROM gaps WHERE session_id='s'").Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "queue") || !strings.Contains(reported, "queue") {
		t.Fatalf("gap reason %q reported %q", reason, reported)
	}
}
