package history

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"f1tele/server/internal/contract"

	"github.com/foxglove/mcap/go/mcap"
)

// RecoveryResult reports what survived a damaged file.
type RecoveryResult struct {
	Artifact   string
	Messages   int
	LastSeq    uint32
	LostTail   bool
	SHA256     string
	SourcePath string
	Error      string
}

// RecoverFile copies every readable message from a damaged MCAP into a new file.
// The original is never modified.
func RecoverFile(source, outDir string) (RecoveryResult, error) {
	res := RecoveryResult{SourcePath: source}
	f, err := os.Open(source)
	if err != nil {
		return res, err
	}
	defer f.Close()
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return res, err
	}
	out := filepath.Join(outDir, fmt.Sprintf("recovered-%s.mcap", newID("rec")))
	of, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return res, err
	}
	w, err := mcap.NewWriter(of, &mcap.WriterOptions{Chunked: true, ChunkSize: 1 << 20, Compression: mcap.CompressionZSTD, IncludeCRC: true})
	if err != nil {
		of.Close()
		return res, err
	}
	w.WriteHeader(&mcap.Header{Profile: "f1tele", Library: "f1tele/1.0.0"})
	w.AddSchema(&mcap.Schema{ID: 1, Name: "f1udp", Encoding: "none", Data: []byte("unchanged received datagrams")})
	lexer, err := mcap.NewLexer(f, &mcap.LexerOptions{EmitChunks: false})
	if err != nil {
		of.Close()
		return res, err
	}
	defer lexer.Close()
	chans := map[string]uint16{}
	schemas := map[uint16]bool{}
	h := sha256.New()
	var buf []byte
	for {
		tok, record, err := lexer.Next(buf)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				res.LostTail = true
				res.Error = err.Error()
			}
			break
		}
		buf = record
		switch tok {
		case mcap.TokenHeader, mcap.TokenFooter, mcap.TokenDataEnd:
			continue
		case mcap.TokenSchema:
			s, err := mcap.ParseSchema(record)
			if err != nil {
				res.LostTail = true
				break
			}
			schemas[s.ID] = true
		case mcap.TokenChannel:
			ch, err := mcap.ParseChannel(record)
			if err != nil {
				res.LostTail = true
				break
			}
			if _, ok := chans[ch.Topic]; !ok {
				id := uint16(len(chans) + 1)
				chans[ch.Topic] = id
				w.AddChannel(&mcap.Channel{ID: id, SchemaID: ch.SchemaID, Topic: ch.Topic, MessageEncoding: ch.MessageEncoding})
			}
		case mcap.TokenMessage:
			m, err := mcap.ParseMessage(record)
			if err != nil {
				res.LostTail = true
				break
			}
			topic := fmt.Sprintf("f1udp/packet/%d", packetID(m.Data))
			// Reconstruct the channel when the original channel record was in
			// a damaged chunk; the raw packet ID is the stable identity.
			id, ok := chans[topic]
			if !ok {
				id = uint16(len(chans) + 1)
				chans[topic] = id
				w.AddChannel(&mcap.Channel{ID: id, SchemaID: 1, Topic: topic, MessageEncoding: "f1udp"})
			}
			if err := w.WriteMessage(&mcap.Message{ChannelID: id, Sequence: m.Sequence, LogTime: m.LogTime, PublishTime: m.PublishTime, Data: m.Data}); err != nil {
				res.LostTail = true
				break
			}
			h.Write(m.Data)
			res.Messages++
			res.LastSeq = m.Sequence
		}
	}
	if err := w.Close(); err != nil {
		return res, err
	}
	if err := of.Sync(); err != nil {
		return res, err
	}
	if err := of.Close(); err != nil {
		return res, err
	}
	res.Artifact = out
	res.SHA256 = hex.EncodeToString(h.Sum(nil))
	return res, nil
}

func packetID(data []byte) uint8 {
	if len(data) > 6 {
		return data[6]
	}
	return 0
}

// VerifyRecording reads every message and returns the payload digest and count.
func VerifyRecording(path string) (int, string, uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", 0, err
	}
	defer f.Close()
	r, err := mcap.NewReader(f)
	if err != nil {
		return 0, "", 0, err
	}
	defer r.Close()
	it, err := r.Messages()
	if err != nil {
		return 0, "", 0, err
	}
	h := sha256.New()
	n := 0
	var last uint32
	for {
		_, _, m, err := it.Next(nil)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return n, hex.EncodeToString(h.Sum(nil)), last, err
		}
		if m == nil {
			break
		}
		h.Write(m.Data)
		n++
		last = m.Sequence
	}
	return n, hex.EncodeToString(h.Sum(nil)), last, nil
}

// RecoveryGap describes the interval between the last catalog checkpoint and
// the first newly captured point.
func RecoveryGap(lastCheckpoint, recovered uint64) contract.Range {
	from := lastCheckpoint + 1
	if recovered < from {
		recovered = from
	}
	return contract.Range{FromSeq: fmt.Sprint(from), ToSeq: fmt.Sprint(recovered)}
}
