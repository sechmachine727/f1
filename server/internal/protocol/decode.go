package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Version identifies the decoder that produced a packet; recorded in MCAP metadata.
const Version = "f1tele-protocol-1.0.0"

type kind uint8

const (
	kU8 kind = iota
	kI8
	kU16
	kI16
	kU32
	kI32
	kU64
	kI64
	kF32
	kF64
	kStr
)

type fieldSpec struct {
	name   string
	kind   kind
	count  int
	strLen int
}

type segmentSpec struct {
	name   string
	count  int
	merge  bool
	width  int
	fields []fieldSpec
}

type fieldLoc struct {
	seg      int
	offset   int
	kind     kind
	isString bool
	size     int
}

type layout struct {
	size int
	segs []segmentSpec
	loc  map[string]fieldLoc
}

var compiled = map[uint16]map[uint8]*layout{}

func init() {
	for format, ids := range layouts {
		compiled[format] = map[uint8]*layout{}
		for pid, segs := range ids {
			l := &layout{segs: segs, loc: map[string]fieldLoc{}}
			off := 0
			for si := range segs {
				seg := &segs[si]
				fo := 0
				for _, f := range seg.fields {
					fsize := kindSize(f.kind) * f.count
					if f.kind == kStr {
						fsize = f.strLen
					}
					if _, exists := l.loc[f.name]; !exists {
						l.loc[f.name] = fieldLoc{seg: si, offset: fo, kind: f.kind, isString: f.kind == kStr, size: fsize}
					}
					fo += fsize
				}
				off += seg.width * seg.count
			}
			l.size = off
			compiled[format][pid] = l
		}
	}
}

func kindSize(k kind) int {
	switch k {
	case kU8, kI8:
		return 1
	case kU16, kI16:
		return 2
	case kU32, kI32, kF32:
		return 4
	case kU64, kI64, kF64:
		return 8
	}
	return 1
}

// Size returns the exact decoded payload length for a format and packet ID.
func Size(format uint16, packetID uint8) (int, bool) {
	s, ok := packetSizes[format][packetID]
	return s, ok
}

// Header is the 29-byte common packet header.
type Header struct {
	Format         uint16
	GameYear       uint8
	MajorVersion   uint8
	MinorVersion   uint8
	PacketVersion  uint8
	PacketID       uint8
	SessionUID     uint64
	SessionTime    float64
	FrameID        uint32
	OverallFrameID uint32
	PlayerCarIndex uint8
	SecondaryIndex uint8
}

// Packet is a validated, zero-copy view over a received datagram.
type Packet struct {
	Header Header
	Data   []byte
	l      *layout
}

// FormatError reports an unknown format or packet ID. The caller keeps the bytes.
type FormatError struct {
	Format uint16
	Packet uint8
	Reason string
}

func (e *FormatError) Error() string { return e.Reason }

func ParseHeader(data []byte) (Header, error) {
	if len(data) < 29 {
		return Header{}, &FormatError{Reason: "header shorter than 29 bytes"}
	}
	return Header{
		Format:         binary.LittleEndian.Uint16(data[0:2]),
		GameYear:       data[2],
		MajorVersion:   data[3],
		MinorVersion:   data[4],
		PacketVersion:  data[5],
		PacketID:       data[6],
		SessionUID:     binary.LittleEndian.Uint64(data[7:15]),
		SessionTime:    float64(math.Float32frombits(binary.LittleEndian.Uint32(data[15:19]))),
		FrameID:        binary.LittleEndian.Uint32(data[19:23]),
		OverallFrameID: binary.LittleEndian.Uint32(data[23:27]),
		PlayerCarIndex: data[27],
		SecondaryIndex: data[28],
	}, nil
}

// Decode validates the header, format, packet ID and minimum length.
// Trailing bytes beyond the documented structure are retained in Data and ignored here.
func Decode(data []byte) (*Packet, error) {
	h, err := ParseHeader(data)
	if err != nil {
		return nil, err
	}
	ids, ok := compiled[h.Format]
	if !ok {
		return nil, &FormatError{Format: h.Format, Packet: h.PacketID, Reason: fmt.Sprintf("unsupported packet format %d", h.Format)}
	}
	l, ok := ids[h.PacketID]
	if !ok {
		return nil, &FormatError{Format: h.Format, Packet: h.PacketID, Reason: fmt.Sprintf("unknown packet id %d for format %d", h.PacketID, h.Format)}
	}
	if len(data) < l.size {
		return nil, &FormatError{Format: h.Format, Packet: h.PacketID, Reason: fmt.Sprintf("packet %d truncated: got %d want %d", h.PacketID, len(data), l.size)}
	}
	return &Packet{Header: h, Data: data, l: l}, nil
}

// Value is one decoded field value.
type Value struct {
	F        float64
	S        string
	IsString bool
}

func (v Value) Int() int64 { return int64(v.F) }
func (v Value) Bool() bool { return v.F != 0 }
func (v Value) Any() any {
	if v.IsString {
		return v.S
	}
	return v.F
}

// Field reads a scalar field from the packet's single-instance segments.
func (p *Packet) Field(name string) (Value, bool) {
	loc, ok := p.l.loc[name]
	if !ok {
		return Value{}, false
	}
	seg := &p.l.segs[loc.seg]
	if seg.count != 1 {
		return Value{}, false
	}
	return p.read(segStart(p.l.segs, loc.seg)+loc.offset, loc)
}

// CarField reads a per-car field from the first array segment that declares it.
func (p *Packet) CarField(name string, car int) (Value, bool) {
	loc, ok := p.l.loc[name]
	if !ok {
		return Value{}, false
	}
	seg := &p.l.segs[loc.seg]
	if seg.count == 1 || car < 0 || car >= seg.count {
		return Value{}, false
	}
	base := segStart(p.l.segs, loc.seg) + car*seg.width
	return p.read(base+loc.offset, loc)
}

// CarCount returns the length of the packet's car array (0 when not per-car).
func (p *Packet) CarCount() int {
	for _, s := range p.l.segs {
		if s.count > 1 {
			return s.count
		}
	}
	return 0
}

func segStart(segs []segmentSpec, idx int) int {
	off := 0
	for i := 0; i < idx; i++ {
		off += segs[i].width * segs[i].count
	}
	return off
}

func (p *Packet) read(off int, loc fieldLoc) (Value, bool) {
	size := loc.size
	if size == 0 {
		size = kindSize(loc.kind)
	}
	if off < 0 || off+size > len(p.Data) {
		return Value{}, false
	}
	b := p.Data[off:]
	switch loc.kind {
	case kU8:
		return Value{F: float64(b[0])}, true
	case kI8:
		return Value{F: float64(int8(b[0]))}, true
	case kU16:
		return Value{F: float64(binary.LittleEndian.Uint16(b))}, true
	case kI16:
		return Value{F: float64(int16(binary.LittleEndian.Uint16(b)))}, true
	case kU32:
		return Value{F: float64(binary.LittleEndian.Uint32(b))}, true
	case kI32:
		return Value{F: float64(int32(binary.LittleEndian.Uint32(b)))}, true
	case kU64:
		return Value{F: float64(binary.LittleEndian.Uint64(b))}, true
	case kI64:
		return Value{F: float64(int64(binary.LittleEndian.Uint64(b)))}, true
	case kF32:
		return Value{F: float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))}, true
	case kF64:
		return Value{F: math.Float64frombits(binary.LittleEndian.Uint64(b))}, true
	case kStr:
		run := b[:size]
		s := string(run)
		if i := strings.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		return Value{S: strings.ToValidUTF8(s, "?"), IsString: true}, true
	}
	return Value{}, false
}

// Values decodes every field of the packet, including array segments,
// for diagnostics and the channel-catalog coverage test.
func (p *Packet) Values() map[string]any {
	out := map[string]any{}
	for si := range p.l.segs {
		seg := &p.l.segs[si]
		base := segStart(p.l.segs, si)
		if seg.count == 1 {
			m := map[string]any{}
			fo := 0
			for _, f := range seg.fields {
				fsize := kindSize(f.kind) * f.count
				if f.kind == kStr {
					fsize = f.strLen
				}
				v, _ := p.read(base+fo, fieldLoc{kind: f.kind, isString: f.kind == kStr, size: fsize})
				if f.count > 1 && f.kind != kStr {
					arr := make([]any, 0, f.count)
					for i := 0; i < f.count; i++ {
						e, _ := p.read(base+fo+i*kindSize(f.kind), fieldLoc{kind: f.kind})
						arr = append(arr, e.Any())
					}
					m[f.name] = arr
				} else {
					m[f.name] = v.Any()
				}
				fo += fsize
			}
			if seg.merge {
				for k, v := range m {
					out[k] = v
				}
			} else {
				out[seg.name] = m
			}
			continue
		}
		items := make([]any, 0, seg.count)
		for i := 0; i < seg.count; i++ {
			item := map[string]any{}
			fo := 0
			b := base + i*seg.width
			for _, f := range seg.fields {
				fsize := kindSize(f.kind) * f.count
				if f.kind == kStr {
					fsize = f.strLen
				}
				v, _ := p.read(b+fo, fieldLoc{kind: f.kind, isString: f.kind == kStr, size: fsize})
				if f.count > 1 && f.kind != kStr {
					arr := make([]any, 0, f.count)
					for i := 0; i < f.count; i++ {
						e, _ := p.read(b+fo+i*kindSize(f.kind), fieldLoc{kind: f.kind})
						arr = append(arr, e.Any())
					}
					item[f.name] = arr
				} else {
					item[f.name] = v.Any()
				}
				fo += fsize
			}
			items = append(items, item)
		}
		out[seg.name] = items
	}
	return out
}

// EventDetails decodes the EventDataDetails union for a known event code.
func (p *Packet) EventDetails() map[string]any {
	if p.Header.PacketID != 3 || len(p.Data) < 33 {
		return nil
	}
	code := p.eventCode()
	d := p.Data[33:]
	out := map[string]any{"code": code}
	u8 := func(i int) int {
		if i < len(d) {
			return int(d[i])
		}
		return 0
	}
	f32 := func(i int) float64 {
		if i+4 > len(d) {
			return 0
		}
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(d[i:])))
	}
	switch code {
	case "FTLP":
		out["vehicleIdx"] = u8(0)
		out["lapTime"] = f32(1)
	case "RTMT":
		out["vehicleIdx"] = u8(0)
		out["reason"] = u8(1)
	case "DRSD":
		out["reason"] = u8(0)
	case "TMPT", "RCWN", "DTSV":
		out["vehicleIdx"] = u8(0)
	case "PENA":
		out["penaltyType"] = u8(0)
		out["infringementType"] = u8(1)
		out["vehicleIdx"] = u8(2)
		out["otherVehicleIdx"] = u8(3)
		out["time"] = u8(4)
		out["lapNum"] = u8(5)
		out["placesGained"] = u8(6)
	case "SPTP":
		out["vehicleIdx"] = u8(0)
		out["speed"] = f32(1)
		out["isOverallFastestInSession"] = u8(5)
		out["isDriverFastestInSession"] = u8(6)
		out["fastestVehicleIdxInSession"] = u8(7)
		out["fastestSpeedInSession"] = f32(8)
	case "STLG":
		out["numLights"] = u8(0)
	case "SGSV":
		out["vehicleIdx"] = u8(0)
		out["stopTime"] = f32(1)
	case "FLBK":
		if len(d) >= 8 {
			out["flashbackFrameIdentifier"] = float64(binary.LittleEndian.Uint32(d[0:]))
			out["flashbackSessionTime"] = f32(4)
		}
	case "BUTN":
		if len(d) >= 4 {
			out["buttonStatus"] = float64(binary.LittleEndian.Uint32(d))
		}
	case "OVTK":
		out["overtakingVehicleIdx"] = u8(0)
		out["beingOvertakenVehicleIdx"] = u8(1)
	case "SCAR":
		out["safetyCarType"] = u8(0)
		out["eventType"] = u8(1)
	case "COLL":
		out["vehicle1Idx"] = u8(0)
		out["vehicle2Idx"] = u8(1)
		out["severity"] = u8(2)
	}
	return out
}

func (p *Packet) eventCode() string {
	if len(p.Data) < 33 {
		return ""
	}
	return string(p.Data[29:33])
}
