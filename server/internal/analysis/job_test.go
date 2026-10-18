package analysis

import (
	"context"
	"os"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

// TestAnalysisJobIsolation proves a cancelled export leaves no artifact and does
// not touch the recording it was reading.
func TestAnalysisJobIsolation(t *testing.T) {
	e, _, sid := fixture(t)
	paths, err := e.Catalog.RecordingsForSession(context.Background(), sid)
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v err %v", paths, err)
	}
	before, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := contract.ExportRequest{SessionID: sid, BranchID: "b", Range: contract.Range{FromSeq: "1", ToSeq: "240"}, Cars: []int{0}, Channels: []string{"car.speed_mps"}, Format: "csv"}
	if path, _, err := e.Export(ctx, req); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Fatalf("cancelled export left %s behind", path)
		}
	}
	after, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("export modified the source recording")
	}
	if n, _, _, err := history.VerifyRecording(paths[0]); err != nil || n != 240 {
		t.Fatalf("source recording changed: n=%d err=%v", n, err)
	}
	// A cancelled query releases quickly rather than scanning to completion.
	start := time.Now()
	_, err = e.Query(ctx, contract.Query{SessionID: sid, BranchID: "b", CarIndices: []int{0}, Channels: []string{"car.speed_mps"}, FromSessionTimeMs: 0, ToSessionTimeMs: 1 << 40, MaxPoints: 100})
	if err == nil {
		t.Fatal("cancelled query returned success")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancelled query did not return promptly")
	}
}
