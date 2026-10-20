package history

import (
	"context"
	"database/sql"
)

// Trace returns the stored decimated series for one lap and channel.
func (c *Catalog) Trace(ctx context.Context, sessionID, branchID string, car, lap int, channel string) ([]byte, error) {
	var points []byte
	err := c.db.QueryRowContext(ctx, `SELECT points FROM lap_traces WHERE session_id=? AND branch_id=? AND car_idx=? AND lap=? AND channel=?`,
		sessionID, branchID, car, lap, channel).Scan(&points)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return points, err
}
