package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// BuildMarshaler is a small fixture generator for tests and the qualification
// workload. Production code only ever decodes received datagrams.
func BuildPacket(format uint16, packetID uint8, h Header, scalar map[string]float64, cars map[int]map[string]float64, strings map[string]string) []byte {
	l := compiled[format][packetID]
	size := l.size
	if packetID == 3 && size < 45 {
		size = 45
	}
	data := make([]byte, size)
	binary.LittleEndian.PutUint16(data[0:], format)
	data[2] = h.GameYear
	data[3] = h.MajorVersion
	data[4] = h.MinorVersion
	data[5] = h.PacketVersion
	data[6] = packetID
	binary.LittleEndian.PutUint64(data[7:], h.SessionUID)
	binary.LittleEndian.PutUint32(data[15:], math.Float32bits(float32(h.SessionTime)))
	binary.LittleEndian.PutUint32(data[19:], h.FrameID)
	binary.LittleEndian.PutUint32(data[23:], h.OverallFrameID)
	data[27] = h.PlayerCarIndex
	data[28] = h.SecondaryIndex
	for si := range l.segs {
		seg := &l.segs[si]
		base := segStart(l.segs, si)
		if seg.count > 1 {
			for car, values := range cars {
				if car >= seg.count {
					continue
				}
				writeFields(data, base+car*seg.width, seg.fields, values, strings, car)
			}
			continue
		}
		writeFields(data, base, seg.fields, scalar, strings, -1)
	}
	return data
}

func writeFields(data []byte, base int, fields []fieldSpec, values map[string]float64, strings map[string]string, car int) {
	off := base
	for _, f := range fields {
		size := kindSize(f.kind) * f.count
		if f.kind == kStr {
			size = f.strLen
		}
		if strings != nil && f.kind == kStr {
			key := f.name
			if car >= 0 {
				key = fmt.Sprintf("%s[%d]", f.name, car)
			}
			if s, ok := strings[key]; ok {
				copy(data[off:off+f.strLen], []byte(s))
			}
		}
		if v, ok := values[f.name]; ok {
			writeValue(data, off, f.kind, v)
		}
		for i := 0; i < f.count; i++ {
			if v, ok := values[fmt.Sprintf("%s[%d]", f.name, i)]; ok {
				writeValue(data, off+i*kindSize(f.kind), f.kind, v)
			}
		}
		off += size
	}
}

func writeValue(data []byte, off int, k kind, v float64) {
	switch k {
	case kU8:
		data[off] = uint8(v)
	case kI8:
		data[off] = uint8(int8(v))
	case kU16:
		binary.LittleEndian.PutUint16(data[off:], uint16(v))
	case kI16:
		binary.LittleEndian.PutUint16(data[off:], uint16(int16(v)))
	case kU32:
		binary.LittleEndian.PutUint32(data[off:], uint32(v))
	case kI32:
		binary.LittleEndian.PutUint32(data[off:], uint32(int32(v)))
	case kU64:
		binary.LittleEndian.PutUint64(data[off:], uint64(v))
	case kI64:
		binary.LittleEndian.PutUint64(data[off:], uint64(int64(v)))
	case kF32:
		binary.LittleEndian.PutUint32(data[off:], math.Float32bits(float32(v)))
	case kF64:
		binary.LittleEndian.PutUint64(data[off:], math.Float64bits(v))
	}
}
