package analytics

import "testing"

func TestDecimateSamplesKeepsEndsAndChanges(t *testing.T) {
	in := make([]sample, 3000)
	for i := range in {
		s := sample{timeMs: int64(i), throttle: 0.5, gear: 5}
		if i >= 1500 {
			s.gear = 6
		}
		if i >= 2000 && i < 2010 {
			s.brake = 0.9
		}
		in[i] = s
	}
	out := decimateSamples(in, MaxSamplesPerLap)
	if len(out) > MaxSamplesPerLap {
		t.Fatalf("not bounded: %d", len(out))
	}
	if out[0][0].(int64) != 0 || out[len(out)-1][0].(int64) != 2999 {
		t.Fatalf("ends lost: %v %v", out[0][0], out[len(out)-1][0])
	}
	seen := map[int64]bool{}
	for _, p := range out {
		seen[p[0].(int64)] = true
	}
	if !seen[1500] || !seen[2000] || !seen[2010] {
		t.Fatalf("shift or brake state dropped: shift=%v brake=%v", seen[1500], seen[2000])
	}
	if out[0][1].(float64) != 0.5 {
		t.Fatalf("value changed: %#v", out[0])
	}
	for i := 1; i < len(out); i++ {
		if out[i][0].(int64) <= out[i-1][0].(int64) {
			t.Fatalf("unordered or duplicated at %d", i)
		}
	}
}

func TestDecimateSamplesShortAndEmpty(t *testing.T) {
	if got := decimateSamples(nil, MaxSamplesPerLap); len(got) != 0 {
		t.Fatalf("empty %#v", got)
	}
	short := []sample{{timeMs: 1, gear: 2}, {timeMs: 2, gear: 3}}
	if got := decimateSamples(short, MaxSamplesPerLap); len(got) != 2 {
		t.Fatalf("short lap lost samples: %d", len(got))
	}
}

func TestDecimateSamplesPathologicalChanges(t *testing.T) {
	// Every sample changes gear, so the protected set alone exceeds the bound.
	in := make([]sample, 2000)
	for i := range in {
		in[i] = sample{timeMs: int64(i), gear: i % 8}
	}
	out := decimateSamples(in, MaxSamplesPerLap)
	if len(out) != MaxSamplesPerLap {
		t.Fatalf("bound %d", len(out))
	}
	if out[0][0].(int64) != 0 || out[len(out)-1][0].(int64) != 1999 {
		t.Fatal("ends lost")
	}
	for i := 1; i < len(out); i++ {
		if out[i][0].(int64) <= out[i-1][0].(int64) {
			t.Fatalf("duplicate or unordered at %d", i)
		}
	}
	if got := decimateSamples(in, 1); len(got) != 1 || got[0][0].(int64) != 0 {
		t.Fatalf("single-point bound %#v", got)
	}
	if got := decimateSamples(in, 2); len(got) != 2 || got[1][0].(int64) != 1999 {
		t.Fatalf("two-point bound %#v", got)
	}
}

