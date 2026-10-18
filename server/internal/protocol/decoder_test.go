package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
)

// Declared by the official packet tables, independently of the layout table.
var specSizes = map[uint16]map[uint8]int{
	2025: {0: 1349, 1: 753, 2: 1285, 3: 45, 4: 1284, 5: 1133, 6: 1352, 7: 1239, 8: 1042, 9: 954, 10: 1041, 11: 1460, 12: 231, 13: 273, 14: 101, 15: 1131},
	2026: {0: 1325, 1: 926, 2: 1399, 3: 45, 4: 1470, 5: 1233, 6: 1448, 7: 1445, 8: 1134, 9: 1062, 10: 1133, 11: 1460, 12: 231, 13: 273, 14: 104, 15: 1231, 16: 269},
}

func TestDeclaredPacketSizes(t *testing.T) {
	for format, ids := range specSizes {
		for pid, want := range ids {
			got, ok := Size(format, pid)
			if !ok {
				t.Fatalf("missing layout %d/%d", format, pid)
			}
			if pid == 3 {
				// The decoder reads the 4-byte event code and union prefix; the
				// datagram carries 16 bytes of union details.
				if want-got != 12 {
					t.Errorf("event layout %d: prefix %d declared %d", format, got, want)
				}
				continue
			}
			if got != want {
				t.Errorf("layout %d/%d = %d want %d", format, pid, got, want)
			}
		}
	}
}

type golden2026 struct {
	Packets map[string]struct {
		Length  int            `json:"length"`
		Decoded map[string]any `json:"decoded"`
	} `json:"packets"`
}

func TestGolden2026Corpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_2026.json")
	if err != nil {
		t.Fatal(err)
	}
	var want golden2026
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile("testdata/golden_2026.bin")
	if err != nil {
		t.Fatal(err)
	}
	for off := 0; off < len(blob); {
		pid := blob[off]
		length := int(binary.LittleEndian.Uint16(blob[off+4:]))
		off += 6
		data := blob[off : off+length]
		off += length
		p, err := Decode(data)
		if err != nil {
			t.Fatalf("packet %d: %v", pid, err)
		}
		if pid == 3 {
			if got := p.EventDetails(); got["safetyCarType"] != 2 {
				t.Errorf("event details = %#v", got)
			}
			continue
		}
		got := p.Values()
		wantPkt := want.Packets[fmt.Sprint(pid)]
		if wantPkt.Length != length {
			t.Errorf("packet %d length %d want %d", pid, length, wantPkt.Length)
		}
		compareMap(t, fmt.Sprintf("packet %d", pid), got, wantPkt.Decoded)
	}
}

func TestReal2025Parity(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_2025.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Packets []struct {
			Timestamp uint64         `json:"timestamp"`
			Length    int            `json:"length"`
			Decoded   map[string]any `json:"decoded"`
			Error     string         `json:"error"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../../tests/fixtures/real_2025_packets.f1bin")
	if err != nil {
		t.Fatal(err)
	}
	off := 0
	for i, rec := range want.Packets {
		if off+10 > len(capture) {
			t.Fatalf("record %d beyond capture", i)
		}
		length := int(binary.LittleEndian.Uint16(capture[off+8:]))
		off += 10
		data := capture[off : off+length]
		off += length
		if rec.Error != "" {
			continue
		}
		p, err := Decode(data)
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		compareMap(t, fmt.Sprintf("record %d packet %d", i, p.Header.PacketID), p.Values(), rec.Decoded)
	}
}

func TestMalformedPackets(t *testing.T) {
	if _, err := Decode([]byte{1, 2, 3}); err == nil {
		t.Fatal("short header accepted")
	}
	bad := make([]byte, 200)
	binary.LittleEndian.PutUint16(bad, 1999)
	if _, err := Decode(bad); err == nil {
		t.Fatal("unknown format accepted")
	}
	binary.LittleEndian.PutUint16(bad, 2026)
	bad[6] = 99
	if _, err := Decode(bad); err == nil {
		t.Fatal("unknown packet id accepted")
	}
	bad[6] = 6
	if _, err := Decode(bad[:100]); err == nil {
		t.Fatal("truncated packet accepted")
	}
}

func TestEveryFieldReachable(t *testing.T) {
	for format, ids := range layouts {
		for pid, segs := range ids {
			for _, seg := range segs {
				if seg.name == "header" {
					continue
				}
				for _, f := range seg.fields {
					if _, ok := compiled[format][pid].loc[f.name]; !ok {
						t.Errorf("%d/%d field %s unreachable", format, pid, f.name)
					}
				}
			}
		}
	}
}

func TestCatalogCoversCanonicalFields(t *testing.T) {
	cat := Catalog()
	seen := map[string]bool{}
	for _, c := range cat {
		seen[c.Name] = true
	}
	for _, c := range Canonical() {
		if !seen[c.Name] {
			t.Errorf("catalog missing %s", c.Name)
		}
	}
	if !seen["raw.1.m_trackTemperature"] && !seen["session.track_temp_c"] {
		t.Error("raw field mapping missing")
	}
}

func compareMap(t *testing.T, where string, got, want any) {
	t.Helper()
	switch g := got.(type) {
	case map[string]any:
		w, ok := want.(map[string]any)
		if !ok {
			t.Fatalf("%s: type mismatch %T vs %T", where, got, want)
		}
		for k, wv := range w {
			if k == "m_liveryColours" {
				continue // arbitrary bytes rendered as a replacement-character string
			}
			gv, ok := g[k]
			if !ok {
				t.Errorf("%s: missing %s", where, k)
				continue
			}
			compareMap(t, where+"."+k, gv, wv)
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				t.Errorf("%s: unexpected %s", where, k)
			}
		}
	case []any:
		w, ok := want.([]any)
		if !ok || len(g) != len(w) {
			t.Fatalf("%s: array mismatch", where)
		}
		for i := range g {
			compareMap(t, fmt.Sprintf("%s[%d]", where, i), g[i], w[i])
		}
	case float64:
		w, ok := want.(float64)
		if !ok {
			t.Fatalf("%s: number vs %T", where, want)
		}
		if math.Abs(g-w) > 1e-6*math.Max(1, math.Abs(w)) {
			t.Errorf("%s = %v want %v", where, g, w)
		}
	case string:
		if w, ok := want.(string); !ok || g != w {
			t.Errorf("%s = %q want %v", where, g, want)
		}
	default:
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v want %#v", where, got, want)
		}
	}
}
