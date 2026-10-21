package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrPromptNotFound = errors.New("prompt revision not found")
	ErrPromptCurrent  = errors.New("current prompt revision cannot be deleted")
)

type PromptRevision struct {
	ID          string `json:"id"`
	Profile     string `json:"profile"`
	Body        string `json:"body"`
	SHA256      string `json:"sha256"`
	CreatedAtMs int64  `json:"created_at_ms"`
	Current     bool   `json:"current"`
}

func (c *Catalog) PromptRevisions(ctx context.Context, profile string) ([]PromptRevision, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT r.id, r.profile, r.body, r.sha256, r.created_at_ms, COALESCE(c.revision_id,'')=r.id
		FROM prompt_revisions r LEFT JOIN prompt_current c ON c.profile=r.profile WHERE r.profile=? ORDER BY r.created_at_ms DESC`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PromptRevision{}
	for rows.Next() {
		var p PromptRevision
		if err := rows.Scan(&p.ID, &p.Profile, &p.Body, &p.SHA256, &p.CreatedAtMs, &p.Current); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (c *Catalog) CurrentPrompt(ctx context.Context, profile string) (*PromptRevision, error) {
	var p PromptRevision
	err := c.db.QueryRowContext(ctx, `SELECT r.id, r.profile, r.body, r.sha256, r.created_at_ms
		FROM prompt_current c JOIN prompt_revisions r ON r.id=c.revision_id WHERE c.profile=?`, profile).
		Scan(&p.ID, &p.Profile, &p.Body, &p.SHA256, &p.CreatedAtMs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Current = true
	return &p, nil
}

func (c *Catalog) CreatePromptRevision(ctx context.Context, profile, body string) (PromptRevision, error) {
	sum := sha256.Sum256([]byte(body))
	p := PromptRevision{ID: newID("pr"), Profile: profile, Body: body, SHA256: hex.EncodeToString(sum[:]), CreatedAtMs: time.Now().UnixMilli(), Current: true}
	c.mu.Lock()
	defer c.mu.Unlock()
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return PromptRevision{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO prompt_revisions(id, profile, body, sha256, created_at_ms) VALUES(?,?,?,?,?)`, p.ID, p.Profile, p.Body, p.SHA256, p.CreatedAtMs); err != nil {
		return PromptRevision{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO prompt_current(profile, revision_id, updated_at_ms) VALUES(?,?,?)
		ON CONFLICT(profile) DO UPDATE SET revision_id=excluded.revision_id, updated_at_ms=excluded.updated_at_ms`, profile, p.ID, p.CreatedAtMs); err != nil {
		return PromptRevision{}, err
	}
	return p, tx.Commit()
}

func (c *Catalog) SelectPromptRevision(ctx context.Context, profile, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exists int
	if err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_revisions WHERE id=? AND profile=?`, id, profile).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("prompt revision %s not found for %s", id, profile)
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO prompt_current(profile, revision_id, updated_at_ms) VALUES(?,?,?)
		ON CONFLICT(profile) DO UPDATE SET revision_id=excluded.revision_id, updated_at_ms=excluded.updated_at_ms`, profile, id, time.Now().UnixMilli())
	return err
}

func (c *Catalog) ResetPrompt(ctx context.Context, profile string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `INSERT INTO prompt_current(profile, revision_id, updated_at_ms) VALUES(?,NULL,?)
		ON CONFLICT(profile) DO UPDATE SET revision_id=NULL, updated_at_ms=excluded.updated_at_ms`, profile, time.Now().UnixMilli())
	return err
}

func (c *Catalog) DeletePromptRevision(ctx context.Context, profile, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var current string
	_ = c.db.QueryRowContext(ctx, `SELECT COALESCE(revision_id,'') FROM prompt_current WHERE profile=?`, profile).Scan(&current)
	if current == id {
		return ErrPromptCurrent
	}
	res, err := c.db.ExecContext(ctx, `DELETE FROM prompt_revisions WHERE id=? AND profile=?`, id, profile)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrPromptNotFound
	}
	return nil
}
