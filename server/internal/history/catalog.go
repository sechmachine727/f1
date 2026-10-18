package history

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/contract"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var migration001 string

//go:embed migrations/002_setup.sql
var migration002 string

const seqBlock = 4096

// Catalog is the SQLite session/catalog store. One connection is shared; the
// writer serializes all writes.
type Catalog struct {
	mu sync.Mutex
	db *sql.DB
}

func OpenCatalog(path string) (*Catalog, error) {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(migration001); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate catalog: %w", err)
	}
	if _, err := db.Exec(migration002); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate setup catalog: %w", err)
	}
	return &Catalog{db: db}, nil
}

func (c *Catalog) Close() error { return c.db.Close() }

func (c *Catalog) DB() *sql.DB { return c.db }

func (c *Catalog) installID(ctx context.Context) (string, error) {
	var id string
	err := c.db.QueryRowContext(ctx, `SELECT value FROM (SELECT recorder_install_id AS value FROM source_contexts LIMIT 1)`).Scan(&id)
	if err == nil && id != "" {
		return id, nil
	}
	return newID("install"), nil
}

// EnsureSourceContext registers a source and durably reserves a sequence block.
func (c *Catalog) EnsureSourceContext(ctx context.Context, id, senderIP string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	install, err := c.installID(ctx)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO source_contexts(id, recorder_install_id, sender_ip, next_reserved_seq)
		VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET sender_ip=excluded.sender_ip`, id, install, senderIP, seqBlock)
	return err
}

// ReserveSeq durably advances the source context's sequence reservation.
func (c *Catalog) ReserveSeq(ctx context.Context, id string, through uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE source_contexts SET next_reserved_seq=? WHERE id=? AND next_reserved_seq<=?`, through+seqBlock, id, through)
	return err
}

// NextSeq returns the next sequence value and reserves blocks ahead.
func (c *Catalog) NextSeq(ctx context.Context, id string) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var next uint64
	if err := tx.QueryRowContext(ctx, `SELECT next_reserved_seq FROM source_contexts WHERE id=?`, id).Scan(&next); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("source context %s not registered", id)
		}
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE source_contexts SET next_reserved_seq=? WHERE id=?`, next+seqBlock, id); err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

type SessionRow struct {
	ID         string
	Context    contract.Context
	StartedAt  int64
	EndedAt    *int64
	Display    string
	ModeFamily string
	CareerID   *string
}

// CreateSession persists the identity that started a live fragment.
func (c *Catalog) CreateSession(ctx context.Context, sessID string, cx contract.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, _ := json.Marshal(cx)
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO sessions(id, source_context_id, regulation_season, career_id, mode_family, track_id, context_json, started_at_ms)
		VALUES(?,?,?,NULL,?,?,?,?)`, sessID, cx.SourceContextID, cx.RegulationSeason, cx.ModeFamily, cx.TrackID, string(raw), time.Now().UnixMilli())
	return err
}

// UpdateSessionContext refreshes evidence that arrives after the first packet
// (track, mode, season and link identifiers).
func (c *Catalog) UpdateSessionContext(ctx context.Context, sessID string, cx contract.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, _ := json.Marshal(cx)
	_, err := c.db.ExecContext(ctx, `UPDATE sessions SET context_json=?, regulation_season=?, mode_family=?, track_id=? WHERE id=?`, string(raw), cx.RegulationSeason, cx.ModeFamily, cx.TrackID, sessID)
	return err
}

func (c *Catalog) CreateFragment(ctx context.Context, f contract.Fragment) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	uid := nullableString(f.GameSessionUID)
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO fragments(id, session_id, source_context_id, game_uid, source, started_at_ms, first_seq, last_seq)
		VALUES(?,?,?,?,?,?,?,?)`, f.ID, f.SessionID, f.SourceContextID, uid, "live", f.StartedAtMs, atoi(f.Range.FromSeq), atoi(f.Range.ToSeq))
	return err
}

func (c *Catalog) CreateBranch(ctx context.Context, b contract.Branch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO branches(id, session_id, parent_id, anchor_seq, anchor_time_ms, last_inherited_seq, is_surviving)
		VALUES(?,?,?,?,?,?,?)`, b.ID, b.SessionID, nullableString(b.ParentID), nullableString(b.AnchorSeq), b.AnchorTimeMs, nullableString(b.LastInheritedSeq), boolInt(b.IsSurviving))
	return err
}

func (c *Catalog) SetSurviving(ctx context.Context, sessionID, branchID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE branches SET is_surviving=0 WHERE session_id=?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE branches SET is_surviving=1 WHERE id=?`, branchID); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Catalog) CreateRecording(ctx context.Context, rec contract.Recording, path string, createdAt int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO recordings(id, fragment_id, path, bytes, state, durable_offset, first_seq, last_seq, sha256, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, rec.ID, rec.FragmentID, path, rec.Bytes, rec.State, rec.Bytes, atoi(rec.Range.FromSeq), atoi(rec.Range.ToSeq), nullableString(rec.SHA256), createdAt)
	return err
}

// Checkpoint advances the durable recording offset and receive range.
func (c *Catalog) Checkpoint(ctx context.Context, recordingID string, offset int64, lastSeq uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE recordings SET durable_offset=?, last_seq=?, bytes=? WHERE id=?`, offset, lastSeq, offset, recordingID)
	return err
}

func (c *Catalog) CloseRecording(ctx context.Context, recordingID string, offset int64, lastSeq uint64, sha string, state string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE recordings SET durable_offset=?, last_seq=?, bytes=?, sha256=?, state=? WHERE id=?`, offset, lastSeq, offset, sha, state, recordingID)
	return err
}

func (c *Catalog) Session(ctx context.Context, id string) (contract.SessionDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var raw string
	var started int64
	var ended sql.NullInt64
	var display, mode string
	err := c.db.QueryRowContext(ctx, `SELECT context_json, started_at_ms, ended_at_ms, display_name, mode_family FROM sessions WHERE id=?`, id).Scan(&raw, &started, &ended, &display, &mode)
	if errors.Is(err, sql.ErrNoRows) {
		return contract.SessionDetail{}, ErrNotFound
	}
	if err != nil {
		return contract.SessionDetail{}, err
	}
	var cx contract.Context
	if err := json.Unmarshal([]byte(raw), &cx); err != nil {
		return contract.SessionDetail{}, err
	}
	d := contract.SessionDetail{Context: cx, Fragments: []contract.Fragment{}, Branches: []contract.Branch{}, Recordings: []contract.Recording{}}
	d.Summary = contract.SessionSummary{
		ID: id, SourceContextID: cx.SourceContextID, RegulationSeason: cx.RegulationSeason,
		ModeFamily: mode, DisplayName: display, TrackID: cx.TrackID, StartedAtMs: started,
		RecordingState: "idle", Completeness: "partial", CareerID: nil,
	}
	if ended.Valid {
		d.Summary.EndedAtMs = &ended.Int64
	}
	rows, err := c.db.QueryContext(ctx, `SELECT id, session_id, source_context_id, game_uid, started_at_ms, ended_at_ms, first_seq, last_seq FROM fragments WHERE session_id=? ORDER BY started_at_ms`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var f contract.Fragment
		var uid sql.NullString
		var start, first, last int64
		var end sql.NullInt64
		if err := rows.Scan(&f.ID, &f.SessionID, &f.SourceContextID, &uid, &start, &end, &first, &last); err != nil {
			return d, err
		}
		f.StartedAtMs = start
		if end.Valid {
			f.EndedAtMs = &end.Int64
		}
		if uid.Valid {
			f.GameSessionUID = &uid.String
		}
		f.Range = contract.Range{FromSeq: strconv.FormatInt(first, 10), ToSeq: strconv.FormatInt(last, 10)}
		d.Fragments = append(d.Fragments, f)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	brows, err := c.db.QueryContext(ctx, `SELECT id, session_id, parent_id, anchor_seq, anchor_time_ms, last_inherited_seq, is_surviving FROM branches WHERE session_id=?`, id)
	if err != nil {
		return d, err
	}
	defer brows.Close()
	for brows.Next() {
		var b contract.Branch
		var parent, anchor, inherited sql.NullString
		var anchorTime sql.NullInt64
		var surviving int
		if err := brows.Scan(&b.ID, &b.SessionID, &parent, &anchor, &anchorTime, &inherited, &surviving); err != nil {
			return d, err
		}
		if parent.Valid {
			b.ParentID = &parent.String
		}
		if anchor.Valid {
			b.AnchorSeq = &anchor.String
		}
		if anchorTime.Valid {
			b.AnchorTimeMs = &anchorTime.Int64
		}
		if inherited.Valid {
			b.LastInheritedSeq = &inherited.String
		}
		b.IsSurviving = surviving == 1
		if b.IsSurviving {
			d.SurvivingBranchID = b.ID
		}
		d.Branches = append(d.Branches, b)
	}
	rrows, err := c.db.QueryContext(ctx, `SELECT r.id, r.fragment_id, r.state, r.bytes, r.sha256, r.first_seq, r.last_seq FROM recordings r JOIN fragments f ON f.id=r.fragment_id WHERE f.session_id=?`, id)
	if err != nil {
		return d, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var rec contract.Recording
		var sha sql.NullString
		var first, last int64
		if err := rrows.Scan(&rec.ID, &rec.FragmentID, &rec.State, &rec.Bytes, &sha, &first, &last); err != nil {
			return d, err
		}
		if sha.Valid {
			rec.SHA256 = &sha.String
		}
		rec.Range = contract.Range{FromSeq: strconv.FormatInt(first, 10), ToSeq: strconv.FormatInt(last, 10)}
		rec.Gaps = []string{}
		d.Recordings = append(d.Recordings, rec)
	}
	return d, nil
}

var ErrNotFound = errors.New("not found")

type SessionFilter struct {
	Cursor     string
	Limit      int
	Season     *int
	ModeFamily string
	TrackID    *int
	CareerID   string
}

func (c *Catalog) ListSessions(ctx context.Context, f SessionFilter) ([]contract.SessionSummary, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	q := `SELECT id, source_context_id, regulation_season, mode_family, career_id, display_name, track_id, started_at_ms, ended_at_ms FROM sessions WHERE 1=1`
	args := []any{}
	if f.Season != nil {
		q += ` AND regulation_season=?`
		args = append(args, *f.Season)
	}
	if f.ModeFamily != "" {
		q += ` AND mode_family=?`
		args = append(args, f.ModeFamily)
	}
	if f.TrackID != nil {
		q += ` AND track_id=?`
		args = append(args, *f.TrackID)
	}
	if f.CareerID != "" {
		q += ` AND career_id=?`
		args = append(args, f.CareerID)
	}
	if f.Cursor != "" {
		q += ` AND started_at_ms < ?`
		args = append(args, atoi(f.Cursor))
	}
	q += ` ORDER BY started_at_ms DESC, id DESC LIMIT ?`
	args = append(args, f.Limit+1)
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []contract.SessionSummary
	for rows.Next() {
		var s contract.SessionSummary
		var season, track, career sql.NullInt64
		var careerID sql.NullString
		var ended sql.NullInt64
		if err := rows.Scan(&s.ID, &s.SourceContextID, &season, &s.ModeFamily, &careerID, &s.DisplayName, &track, &s.StartedAtMs, &ended); err != nil {
			return nil, "", err
		}
		if season.Valid {
			v := int(season.Int64)
			s.RegulationSeason = &v
		}
		if track.Valid {
			v := int(track.Int64)
			s.TrackID = &v
		}
		if careerID.Valid {
			s.CareerID = &careerID.String
		}
		if career.Valid {
			_ = career
		}
		if ended.Valid {
			s.EndedAtMs = &ended.Int64
		}
		s.RecordingState = "idle"
		s.Completeness = "partial"
		out = append(out, s)
	}
	next := ""
	if len(out) > f.Limit {
		last := out[f.Limit-1]
		next = strconv.FormatInt(last.StartedAtMs, 10)
		out = out[:f.Limit]
	}
	return out, next, rows.Err()
}

func (c *Catalog) PatchSession(ctx context.Context, id string, display *string, career *string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if display != nil {
		if _, err := c.db.ExecContext(ctx, `UPDATE sessions SET display_name=? WHERE id=?`, *display, id); err != nil {
			return err
		}
	}
	if career != nil {
		if _, err := c.db.ExecContext(ctx, `UPDATE sessions SET career_id=? WHERE id=?`, *career, id); err != nil {
			return err
		}
	}
	return nil
}

func (c *Catalog) InsertLap(ctx context.Context, l contract.LapSummary) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	validity, _ := json.Marshal(map[string]any{"valid": l.Valid, "pit": l.PitLap, "neutralized": l.Neutralized, "complete": l.Complete})
	summary, _ := json.Marshal(map[string]any{"compound": l.Compound, "conditions": l.Conditions})
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO laps(id, session_id, branch_id, car_idx, lap, start_seq, end_seq, time_ms, validity_json, summary_json)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, l.ID, l.SessionID, l.BranchID, l.CarIndex, l.Lap, atoi(l.StartSeq), atoi(l.EndSeq), l.LapTimeMs.Value, string(validity), string(summary))
	return err
}

func (c *Catalog) Laps(ctx context.Context, sessionID, branchID string, car *int) ([]contract.LapSummary, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q := `SELECT id, session_id, branch_id, car_idx, lap, start_seq, end_seq, time_ms, validity_json, summary_json FROM laps WHERE session_id=? AND branch_id=?`
	args := []any{sessionID, branchID}
	if car != nil {
		q += ` AND car_idx=?`
		args = append(args, *car)
	}
	q += ` ORDER BY car_idx, lap`
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contract.LapSummary
	for rows.Next() {
		var l contract.LapSummary
		var timeMs sql.NullInt64
		var validity, summary string
		if err := rows.Scan(&l.ID, &l.SessionID, &l.BranchID, &l.CarIndex, &l.Lap, &l.StartSeq, &l.EndSeq, &timeMs, &validity, &summary); err != nil {
			return nil, err
		}
		if timeMs.Valid {
			l.LapTimeMs = contract.Available(timeMs.Int64, "ms", "catalog.laps")
		} else {
			l.LapTimeMs = contract.Unavailable("invalid", "ms", "catalog.laps", "missing lap time")
		}
		l.DistanceM = contract.Unavailable("not_received", "m", "catalog.laps", "distance is not catalogued")
		var sm struct {
			Compound   contract.Measurement `json:"compound"`
			Conditions string               `json:"conditions"`
		}
		_ = json.Unmarshal([]byte(summary), &sm)
		l.Compound = sm.Compound
		if l.Compound.SourceID == "" {
			l.Compound = contract.Unavailable("not_received", "compound", "catalog.laps", "compound not recorded")
		}
		l.Conditions = sm.Conditions
		var v map[string]bool
		_ = json.Unmarshal([]byte(validity), &v)
		l.Valid, l.PitLap, l.Neutralized, l.Complete = v["valid"], v["pit"], v["neutralized"], v["complete"]
		l.Gaps = []string{}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (c *Catalog) InsertEvent(ctx context.Context, sessionID, branchID string, e contract.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, _ := json.Marshal(e)
	_, err := c.db.ExecContext(ctx, `INSERT OR REPLACE INTO events(id, session_id, branch_id, seq, type, severity, payload_json) VALUES(?,?,?,?,?,?,?)`, e.ID, sessionID, branchID, atoi(e.Seq), e.Type, e.Severity, string(raw))
	return err
}

func (c *Catalog) InsertGap(ctx context.Context, sessionID, branchID string, from, to uint64, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `INSERT INTO gaps(id, session_id, branch_id, from_seq, to_seq, reason) VALUES(?,?,?,?,?,?)`, newID("gap"), sessionID, branchID, from, to, reason)
	return err
}

func (c *Catalog) DeleteRecordingRow(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `DELETE FROM recordings WHERE id=?`, id)
	return err
}

func newID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func nullableString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func atoi(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
