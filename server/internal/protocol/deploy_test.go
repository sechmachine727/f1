package protocol

import "testing"

func TestDeployModeLabelForSeason(t *testing.T) {
	y2025, y2026, other := 2025, 2026, 2024
	cases := []struct {
		mode   int
		season *int
		want   string
	}{
		{3, &y2025, "Overtake"},
		{3, &y2026, "Boost"},
		{3, nil, "Mode 3 (season unknown)"},
		{3, &other, "Mode 3 (season unknown)"},
		{0, &y2026, "None"},
		{1, &y2025, "Medium"},
		{2, &y2026, "Hotlap"},
	}
	for _, c := range cases {
		if got := DeployModeLabelForSeason(float64(c.mode), c.season); got != c.want {
			t.Fatalf("mode %d season %v = %q, want %q", c.mode, c.season, got, c.want)
		}
	}
	if got := DeployModeLabel(3); got != "Overtake" {
		t.Fatalf("legacy label %q", got)
	}
}

