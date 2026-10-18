package state

import (
	"testing"
	"time"

	"f1tele/server/internal/protocol"
)

func BenchmarkApply(b *testing.B) {
	r := New("ctx", func() time.Time { return time.Unix(1_700_000_000, 0) })
	data := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 1}, nil, map[int]map[string]float64{0: {"m_speed": 300}}, nil)
	p, err := protocol.Decode(data)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Apply(uint64(i+1), time.Unix(1_700_000_000, 0), data, p, "")
	}
}

func BenchmarkDecode(b *testing.B) {
	data := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 1}, nil, map[int]map[string]float64{0: {"m_speed": 300}}, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := protocol.Decode(data); err != nil {
			b.Fatal(err)
		}
	}
}
