package analytics

import "testing"

func TestCompactSamplesKeepsWholeLap(t *testing.T) {
	in := make([]sample, 4001)
	for i := range in {
		in[i] = sample{timeMs: int64(i), gear: 7}
	}
	in[2500].gear = 6
	in[2600].brake = 1
	in[2650].brake = 0
	out := compactSamples(in)
	if len(out) >= len(in) {
		t.Fatalf("not compacted: %d", len(out))
	}
	if out[0].timeMs != 0 || out[len(out)-1].timeMs != 4000 {
		t.Fatalf("ends lost: %d..%d", out[0].timeMs, out[len(out)-1].timeMs)
	}
	seen := map[int64]bool{}
	for _, s := range out {
		seen[s.timeMs] = true
	}
	if !seen[2500] || !seen[2600] || !seen[2650] {
		t.Fatalf("state change lost: %v %v %v", seen[2500], seen[2600], seen[2650])
	}
	for i := 1; i < len(out); i++ {
		if out[i].timeMs <= out[i-1].timeMs {
			t.Fatalf("unordered at %d", i)
		}
	}
}

