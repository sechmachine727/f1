package analysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/foxglove/mcap/go/mcap"
)

func TestScanUnfinalizedRecording(t *testing.T) {
	path := filepath.Join(t.TempDir(), "active.mcap")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w, err := mcap.NewWriter(f, &mcap.WriterOptions{Chunked: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHeader(&mcap.Header{Library: "test"}); err != nil {
		t.Fatal(err)
	}
	w.AddChannel(&mcap.Channel{ID: 1, Topic: "udp", MessageEncoding: "binary"})
	if err := w.WriteMessage(&mcap.Message{ChannelID: 1, Sequence: 7, LogTime: 1, Data: []byte("hello")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Drop the footer and trailing magic so the file looks like an active one.
	if err := os.WriteFile(path, data[:len(data)-28], 0o644); err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := scanFile(context.Background(), path, func(m *mcap.Message) error {
		if string(m.Data) != "hello" || m.Sequence != 7 {
			t.Fatalf("message %#v", m)
		}
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("messages = %d", n)
	}
}
