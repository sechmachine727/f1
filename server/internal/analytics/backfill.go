package analytics

import (
	"context"
	"io"
	"log/slog"
	"os"

	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"

	"github.com/foxglove/mcap/go/mcap"
)

// Backfill derives analytics for existing recordings. It never rewrites a
// recording, skips sessions that already have a summary, and is safe to rerun.
func Backfill(ctx context.Context, cat *history.Catalog, logger *slog.Logger, limit int) {
	rows, err := cat.DB().QueryContext(ctx, `SELECT s.id FROM sessions s
		LEFT JOIN session_analytics a ON a.session_id=s.id
		WHERE a.session_id IS NULL AND EXISTS (SELECT 1 FROM laps l WHERE l.session_id=s.id)
		ORDER BY s.started_at_ms DESC LIMIT ?`, limit)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		detail, err := cat.Session(ctx, id)
		if err != nil {
			continue
		}
		paths, err := cat.RecordingsForSession(ctx, id)
		if err != nil || len(paths) == 0 {
			continue
		}
		tr := NewTracker(cat)
		length := 5000.0
		for _, path := range paths {
			scan(path, func(seq uint64, p *protocol.Packet) {
				tr.Observe(ctx, seq, p, detail.Context, map[string]float64{"track_length_m": length})
			})
		}
		if err := tr.Close(ctx); err != nil {
			if logger != nil {
				logger.Warn("analytics backfill", "session", id, "err", err)
			}
		}
	}
}

func scan(path string, fn func(uint64, *protocol.Packet)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	r, err := mcap.NewReader(f)
	if err != nil {
		return
	}
	defer r.Close()
	it, err := r.Messages()
	if err != nil {
		return
	}
	for {
		_, _, m, err := it.Next(nil)
		if err == io.EOF || m == nil {
			return
		}
		if err != nil {
			return
		}
		p, derr := protocol.Decode(m.Data)
		if derr != nil {
			continue
		}
		fn(uint64(m.Sequence), p)
	}
}
