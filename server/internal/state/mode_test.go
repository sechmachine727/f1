package state

import "testing"

// Values come from the Game Mode and Session type tables in the supplied
// 2026 v1.2 PDF appendix.
func TestModeFamiliesMatchPublishedTable(t *testing.T) {
	cases := []struct {
		mode, session int
		want          string
	}{
		{0, 0, "unknown"},
		{4, 15, "solo"},
		{5, 18, "time_trial"},
		{6, 15, "solo"},
		{7, 15, "multiplayer"},
		{15, 15, "multiplayer"},
		{17, 15, "solo"},
		{27, 15, "career"},
		{28, 15, "career"},
		{29, 15, "multiplayer"},
		{30, 15, "career"},
		{75, 15, "solo"},
		{127, 15, "other"},
		{0, 18, "time_trial"},
		{99, 15, "unknown"},
	}
	for _, c := range cases {
		if got := modeFamily(c.mode, c.session); got != c.want {
			t.Errorf("mode %d session %d = %s want %s", c.mode, c.session, got, c.want)
		}
	}
}
