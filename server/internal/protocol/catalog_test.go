package protocol

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestChannelCatalogGolden pins the published channel surface. Regenerating the
// file is a deliberate contract change, so the diff is reviewable.
func TestChannelCatalogGolden(t *testing.T) {
	const path = "testdata/channel_catalog.json"
	got, err := json.MarshalIndent(Catalog(), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden: %v", err)
	}
	if string(got) != strings.TrimRight(string(want), "\n") {
		t.Fatal("channel catalog drifted; review and regenerate with UPDATE_GOLDEN=1")
	}
	var entries []struct {
		Name      string   `json:"name"`
		Unit      string   `json:"unit"`
		Source    []string `json:"source_fields"`
		Rule      string   `json:"availability_rule"`
		Frequency string   `json:"frequency_class"`
	}
	if err := json.Unmarshal(want, &entries); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Name == "" || len(e.Source) == 0 || e.Rule == "" || e.Frequency == "" {
			t.Fatalf("incomplete channel entry %#v", e)
		}
		seen[e.Name] = true
	}
	for _, c := range Canonical() {
		if !seen[c.Name] {
			t.Fatalf("canonical channel %s missing from golden", c.Name)
		}
	}
	if len(entries) < 200 {
		t.Fatalf("catalog too small: %d raw and canonical entries", len(entries))
	}
}
