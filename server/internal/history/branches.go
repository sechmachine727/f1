package history

import (
	"context"
	"database/sql"
	"errors"
)

// Segment is one branch's ownership of a receive-sequence interval.
type Segment struct {
	BranchID string
	FromSeq  uint64
	ToSeq    uint64 // 0 means open-ended (latest durable)
}

// InheritedSegments resolves a branch's surviving prefix: parent data strictly
// before the anchor, then child data from the anchor onward.
func (c *Catalog) InheritedSegments(ctx context.Context, branchID string) ([]Segment, error) {
	var segs []Segment
	seen := map[string]bool{}
	childAnchor := uint64(0)
	for branchID != "" {
		if seen[branchID] {
			return nil, errors.New("branch ancestry cycle")
		}
		seen[branchID] = true
		var parent sql.NullString
		var anchor sql.NullInt64
		if err := c.db.QueryRowContext(ctx, "SELECT parent_id, anchor_seq FROM branches WHERE id=?", branchID).Scan(&parent, &anchor); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		seg := Segment{BranchID: branchID}
		if anchor.Valid {
			seg.FromSeq = uint64(anchor.Int64)
		}
		if childAnchor > 0 {
			seg.ToSeq = childAnchor - 1
		}
		segs = append([]Segment{seg}, segs...)
		if !parent.Valid {
			break
		}
		childAnchor = seg.FromSeq
		branchID = parent.String
	}
	return segs, nil
}

// RecordingsForSession returns owned recording files in sequence order so a
// recovered older chunk cannot replay after a newer one.
func (c *Catalog) RecordingsForSession(ctx context.Context, sessionID string) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT r.path FROM recordings r JOIN fragments f ON f.id=r.fragment_id WHERE f.session_id=? ORDER BY r.first_seq, r.created_at_ms, r.rowid", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordingRange binds a file to the fragment and receive-sequence span it
// covers, so exports can label rows with the fragment they came from.
type RecordingRange struct {
	Path       string
	FragmentID string
	FirstSeq   uint64
	LastSeq    uint64
}

func (c *Catalog) RecordingRanges(ctx context.Context, sessionID string) ([]RecordingRange, error) {
	rows, err := c.db.QueryContext(ctx, "SELECT r.path, r.fragment_id, r.first_seq, r.last_seq FROM recordings r JOIN fragments f ON f.id=r.fragment_id WHERE f.session_id=? ORDER BY r.first_seq, r.created_at_ms, r.rowid", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecordingRange
	for rows.Next() {
		var rr RecordingRange
		var first, last int64
		if err := rows.Scan(&rr.Path, &rr.FragmentID, &first, &last); err != nil {
			return nil, err
		}
		rr.FirstSeq, rr.LastSeq = uint64(first), uint64(last)
		out = append(out, rr)
	}
	return out, rows.Err()
}
