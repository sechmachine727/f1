package history

import (
	"path/filepath"
	"testing"
)

// The branch_strategy table is the durable home for the two cursors. A branch
// change must reset them without deleting the prior accepted output, which is
// what post-race review relies on.
func TestBranchStrategyCursors(t *testing.T) {
	cat, err := OpenCatalog(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	for _, q := range []string{
		"INSERT INTO source_contexts(id, recorder_install_id, sender_ip, next_reserved_seq) VALUES('c','i','ip',4096)",
		"INSERT INTO sessions(id, source_context_id, mode_family, context_json, started_at_ms) VALUES('s','c','solo','{}',1)",
		"INSERT INTO branches(id, session_id, is_surviving) VALUES('b1','s',1)",
		"INSERT INTO branches(id, session_id, is_surviving) VALUES('b2','s',0)",
	} {
		if _, err := cat.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cat.DB().Exec("INSERT INTO branch_strategy(branch_id, dispatch_cursor_json, evidence_cursor, accepted_invocation_id) VALUES(?,?,?,?)", "b1", `{"rule:r":4}`, 120, "inv-1"); err != nil {
		t.Fatal(err)
	}
	var dispatch string
	var evidence int
	var accepted string
	if err := cat.DB().QueryRow("SELECT dispatch_cursor_json, evidence_cursor, accepted_invocation_id FROM branch_strategy WHERE branch_id='b1'").Scan(&dispatch, &evidence, &accepted); err != nil {
		t.Fatal(err)
	}
	if dispatch != `{"rule:r":4}` || evidence != 120 || accepted != "inv-1" {
		t.Fatalf("cursors %s %d %s", dispatch, evidence, accepted)
	}
	// A new branch gets its own row and starts empty.
	if _, err := cat.DB().Exec("INSERT INTO branch_strategy(branch_id) VALUES(?)", "b2"); err != nil {
		t.Fatal(err)
	}
	var e2 *int64
	var a2 *string
	if err := cat.DB().QueryRow("SELECT evidence_cursor, accepted_invocation_id FROM branch_strategy WHERE branch_id='b2'").Scan(&e2, &a2); err != nil {
		t.Fatal(err)
	}
	if e2 != nil || a2 != nil {
		t.Fatal("new branch inherited a cursor")
	}
}
