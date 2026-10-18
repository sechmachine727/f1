package history

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"f1tele/server/internal/contract"

	"github.com/foxglove/mcap/go/mcap"
)

// ImportResult reports faithful legacy import coverage.
type ImportResult struct {
	SessionID     string
	RecordingID   string
	Artifact      string
	Records       int
	FirstSeq      uint64
	LastSeq       uint64
	TruncatedTail bool
	Note          string
}

// ImportF1Bin reads legacy <QH records, preserves timestamps and bytes, and
// writes a new MCAP artifact. The source file is never modified.
func ImportF1Bin(ctx context.Context, cat *Catalog, source, dataDir, sourceCtx, senderIP string) (ImportResult, error) {
	res := ImportResult{Note: "legacy captures may contain sampled high-rate packets; timestamps and bytes are preserved as received"}
	raw, err := os.ReadFile(source)
	if err != nil {
		return res, err
	}
	if err := cat.EnsureSourceContext(ctx, sourceCtx, senderIP); err != nil {
		return res, err
	}
	now := time.Now().UnixMilli()
	res.SessionID = newID("ses")
	res.RecordingID = newID("rec")
	branchID := newID("br")
	fragID := newID("frg")
	cx := contract.Context{
		SessionID: res.SessionID, FragmentID: fragID, BranchID: branchID, SourceKind: "import",
		ModeFamily: "unknown", SourceContextID: sourceCtx, SeasonEvidence: []string{},
	}
	if err := cat.CreateSession(ctx, res.SessionID, cx); err != nil {
		return res, err
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: fragID, SessionID: res.SessionID, SourceContextID: sourceCtx, StartedAtMs: now, Range: contract.Range{FromSeq: "0", ToSeq: "0"}}); err != nil {
		return res, err
	}
	if err := cat.CreateBranch(ctx, contract.Branch{ID: branchID, SessionID: res.SessionID, IsSurviving: true}); err != nil {
		return res, err
	}
	dir := filepath.Join(dataDir, "imports", res.SessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return res, err
	}
	path := filepath.Join(dir, res.RecordingID+".mcap")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return res, err
	}
	w, err := mcap.NewWriter(f, &mcap.WriterOptions{Chunked: true, ChunkSize: 1 << 20, Compression: mcap.CompressionZSTD, IncludeCRC: true})
	if err != nil {
		f.Close()
		return res, err
	}
	w.WriteHeader(&mcap.Header{Profile: "f1tele", Library: "f1tele/1.0.0"})
	w.AddSchema(&mcap.Schema{ID: 1, Name: "f1udp", Encoding: "none", Data: []byte("legacy .f1bin import")})
	off := 0
	seq := uint64(1)
	channels := map[uint8]uint16{}
	for {
		if off+10 > len(raw) {
			if off != len(raw) {
				res.TruncatedTail = true
			}
			break
		}
		ts := binary.LittleEndian.Uint64(raw[off:])
		length := int(binary.LittleEndian.Uint16(raw[off+8:]))
		off += 10
		if off+length > len(raw) {
			res.TruncatedTail = true
			break
		}
		data := raw[off : off+length]
		off += length
		pid := uint8(0)
		if len(data) > 6 {
			pid = data[6]
		}
		ch, ok := channels[pid]
		if !ok {
			ch = uint16(len(channels)) + 1
			channels[pid] = ch
			w.AddChannel(&mcap.Channel{ID: ch, SchemaID: 1, Topic: fmt.Sprintf("f1udp/packet/%d", pid), MessageEncoding: "f1udp"})
		}
		if err := w.WriteMessage(&mcap.Message{ChannelID: ch, Sequence: uint32(seq), LogTime: ts, PublishTime: ts, Data: data}); err != nil {
			f.Close()
			return res, err
		}
		if res.FirstSeq == 0 {
			res.FirstSeq = seq
		}
		res.LastSeq = seq
		seq++
		res.Records++
	}
	if err := w.Close(); err != nil {
		return res, err
	}
	if err := f.Sync(); err != nil {
		return res, err
	}
	if err := f.Close(); err != nil {
		return res, err
	}
	n, _, last, err := VerifyRecording(path)
	if err != nil {
		return res, err
	}
	if n != res.Records {
		return res, fmt.Errorf("import verification: wrote %d records, read %d", res.Records, n)
	}
	res.LastSeq = uint64(last)
	res.Artifact = path
	rec := contract.Recording{ID: res.RecordingID, FragmentID: fragID, State: "complete", Bytes: fileSize(path), Range: contract.Range{FromSeq: fmt.Sprint(res.FirstSeq), ToSeq: fmt.Sprint(res.LastSeq)}, Gaps: []string{}}
	if err := cat.CreateRecording(ctx, rec, path, now); err != nil {
		return res, err
	}
	return res, nil
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}
