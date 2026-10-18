package server

import "testing"

// The actual-compound table in the supplied UDP appendix. Visual compounds use
// a different numbering, which is why 16..22 must not read as soft/medium/hard.
func TestCompoundNames(t *testing.T) {
	cases := map[int]string{
		7: "Inter", 8: "Wet", 9: "Dry (classic)", 10: "Wet (classic)",
		11: "Super soft", 12: "Soft", 13: "Medium", 14: "Hard", 15: "Wet (F2)",
		16: "C5", 17: "C4", 18: "C3", 19: "C2", 20: "C1", 21: "C0", 22: "C6",
		0: "Compound 0", 99: "Compound 99",
	}
	for in, want := range cases {
		if got := compoundName(in); got != want {
			t.Errorf("compound %d = %q want %q", in, got, want)
		}
	}
}
