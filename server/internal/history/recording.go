package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"f1tele/server/internal/contract"

	"github.com/foxglove/mcap/go/mcap"
)

// RecordingState mirrors the health contract.
const (
	RecIdle         = "idle"
	RecRecording    = "recording"
	RecStoppedUser  = "stopped_user"
	RecStoppedLimit = "stopped_limit"
	RecStoppedError = "stopped_error"
)

type DiskFree func(path string) (free, total int64, err error)

type recorderItem struct {
	seq   uint64
	at    time.Time
	data  []byte
	valid bool
}

// Recorder writes every received datagram to chunked, CRC-protected MCAP.
// A bounded byte budget isolates live ingestion from disk stalls.
type Recorder struct {
	mu        sync.Mutex
	cfg       contract.RecordingConfig
	catalog   *Catalog
	dir       string
	queue     chan recorderItem
	queued    int64
	state     string
	reason    string
	stopCh    chan struct{}
	done      chan struct{}
	diskFree  DiskFree
	now       func() time.Time
	warn      atomic.Int64
	rotateReq atomic.Bool

	sessionID        string
	fragmentID       string
	sourceCtx        string
	recordingID      string
	file             *os.File
	writer           *mcap.Writer
	countWriter      *countingWriter
	fileBytes        int64
	channels         map[uint8]uint16
	hasher           hash.Hash
	lastSeq          uint64
	firstSeq         uint64
	startedAt        int64
	rotation         int
	lastCheckpoint   int64
	lastCheckpointAt time.Time
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// The payload digest streams; retaining every datagram would cost gigabytes
// over a two-hour race.

// NewRecorder builds a recorder; Start must be called before Record.
func NewRecorder(cfg contract.RecordingConfig, catalog *Catalog, dir string, disk DiskFree) *Recorder {
	if disk == nil {
		disk = realDiskFree
	}
	return &Recorder{
		cfg: cfg, catalog: catalog, dir: dir, diskFree: disk,
		queue: make(chan recorderItem, 8192), channels: map[uint8]uint16{},
		state: RecIdle, stopCh: make(chan struct{}), done: make(chan struct{}), now: time.Now,
	}
}

func (r *Recorder) Start() {
	if !r.cfg.Enabled {
		r.setState(RecIdle, "recording disabled in configuration")
		close(r.done)
		return
	}
	r.setState(RecRecording, "")
	go r.loop()
}

func (r *Recorder) SetIdentity(sessionID, fragmentID, sourceCtx string) {
	r.mu.Lock()
	changed := r.fragmentID != "" && r.fragmentID != fragmentID
	r.sessionID, r.fragmentID, r.sourceCtx = sessionID, fragmentID, sourceCtx
	r.mu.Unlock()
	if changed {
		r.rotateReq.Store(true)
	}
}

// Record enqueues a datagram. It never blocks ingestion: an exhausted budget
// stops recording with a visible reason instead.
func (r *Recorder) Record(seq uint64, at time.Time, data []byte) {
	r.mu.Lock()
	if r.state != RecRecording {
		r.mu.Unlock()
		return
	}
	if r.queued+int64(len(data)) > r.cfg.QueueBytes {
		r.state = RecStoppedLimit
		r.reason = "recording queue exhausted; interval not recorded"
		r.mu.Unlock()
		return
	}
	r.queued += int64(len(data))
	r.mu.Unlock()
	select {
	case r.queue <- recorderItem{seq: seq, at: at, data: append([]byte(nil), data...)}:
	default:
		r.mu.Lock()
		r.queued -= int64(len(data))
		r.state = RecStoppedLimit
		r.reason = "recording queue full; interval not recorded"
		r.mu.Unlock()
	}
}

func (r *Recorder) loop() {
	defer close(r.done)
	flush := time.NewTicker(time.Duration(max(1, r.cfg.ChunkFlushMs)) * time.Millisecond)
	defer flush.Stop()
	for {
		select {
		case item := <-r.queue:
			r.write(item)
			r.mu.Lock()
			r.queued -= int64(len(item.data))
			r.mu.Unlock()
		case <-flush.C:
			r.checkpoint()
		case <-r.stopCh:
			for {
				select {
				case item := <-r.queue:
					r.write(item)
				default:
					r.finish()
					return
				}
			}
		}
	}
}

func (r *Recorder) write(item recorderItem) {
	r.mu.Lock()
	identity := [3]string{r.sessionID, r.fragmentID, r.sourceCtx}
	r.mu.Unlock()
	if r.rotateReq.Swap(false) {
		r.finishFile("complete")
	}
	r.mu.Lock()
	state := r.state
	r.mu.Unlock()
	if state != RecRecording {
		return
	}
	if err := r.ensureOpen(item, identity); err != nil {
		r.fail(err)
		return
	}
	pid := uint8(0)
	if len(item.data) > 6 {
		pid = item.data[6]
	}
	ch, ok := r.channels[pid]
	if !ok {
		ch = uint16(len(r.channels)) + 1
		r.channels[pid] = ch
		r.writer.AddChannel(&mcap.Channel{ID: ch, SchemaID: 1, Topic: fmt.Sprintf("f1udp/packet/%d", pid), MessageEncoding: "f1udp"})
	}
	if err := r.writer.WriteMessage(&mcap.Message{
		ChannelID: ch, Sequence: uint32(item.seq), LogTime: uint64(item.at.UnixNano()),
		PublishTime: uint64(item.at.UnixNano()), Data: item.data,
	}); err != nil {
		r.fail(err)
		return
	}
	r.hasher.Write(item.data)
	if r.firstSeq == 0 {
		r.firstSeq = item.seq
	}
	if item.seq > r.lastSeq || r.lastSeq == 0 {
		r.lastSeq = item.seq
	}
	if r.countWriter.n >= r.cfg.RotateBytes {
		r.rotate()
		// rotate closes the file and clears the writer; the current datagram is
		// already in the closed file, and the next write opens the new one.
		return
	}
	// Checkpoint on new bytes only. Comparing cumulative size made every write
	// fsync once the file passed one chunk, which starved the writer at 2 kHz.
	if r.countWriter.n-r.lastCheckpoint >= r.cfg.ChunkBytes &&
		r.now().Sub(r.lastCheckpointAt) >= time.Duration(max(1, r.cfg.ChunkFlushMs))*time.Millisecond {
		r.checkpoint()
	}
}

func (r *Recorder) ensureOpen(item recorderItem, identity [3]string) error {
	if r.file != nil {
		return nil
	}
	if identity[0] == "" || identity[1] == "" {
		return errors.New("recording identity unavailable")
	}
	free, _, err := r.diskFree(r.dir)
	if err != nil {
		return fmt.Errorf("disk free check: %w", err)
	}
	used, err := r.usedBytes()
	if err != nil {
		return err
	}
	if used+int64(len(item.data)) > r.cfg.CapBytes {
		return fmt.Errorf("storage cap reached (%d bytes); recording stopped", r.cfg.CapBytes)
	}
	if free-int64(len(item.data)) < r.cfg.FreeReserveBytes {
		return fmt.Errorf("free reserve reached (%d bytes remaining); recording stopped", free)
	}
	dir := filepath.Join(r.dir, "recordings", sanitize(identity[0]), sanitize(identity[1]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	r.rotation++
	r.recordingID = newID("rec")
	path := filepath.Join(dir, fmt.Sprintf("%s-%02d.mcap", r.recordingID, r.rotation))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	r.file = f
	r.countWriter = &countingWriter{w: f}
	w, err := mcap.NewWriter(r.countWriter, &mcap.WriterOptions{
		Chunked: true, ChunkSize: r.cfg.ChunkBytes, Compression: mcap.CompressionZSTD,
		IncludeCRC: true, CompressionLevel: mcap.CompressionLevelFastest,
	})
	if err != nil {
		f.Close()
		return err
	}
	r.writer = w
	r.channels = map[uint8]uint16{}
	r.startedAt = r.now().UnixMilli()
	w.WriteHeader(&mcap.Header{Profile: "f1tele", Library: "f1tele/1.0.0"})
	w.AddSchema(&mcap.Schema{ID: 1, Name: "f1udp", Encoding: "none", Data: []byte("unchanged received datagrams")})
	r.firstSeq, r.lastSeq = 0, 0
	r.lastCheckpoint = 0
	r.lastCheckpointAt = time.Time{}
	r.hasher = sha256.New()
	rec := contract.Recording{ID: r.recordingID, FragmentID: identity[1], State: "active", Range: contract.Range{FromSeq: "0", ToSeq: "0"}, Gaps: []string{}}
	if err := r.catalog.CreateRecording(context.Background(), rec, path, r.startedAt); err != nil {
		return err
	}
	return nil
}

func (r *Recorder) usedBytes() (int64, error) {
	var sum int64
	err := r.catalog.DB().QueryRow(`SELECT COALESCE(SUM(bytes),0) FROM recordings`).Scan(&sum)
	return sum, err
}

func (r *Recorder) checkpoint() {
	if r.file == nil || r.writer == nil {
		return
	}
	if r.countWriter.n == r.lastCheckpoint && !r.lastCheckpointAt.IsZero() {
		return
	}
	if err := r.file.Sync(); err != nil {
		r.fail(err)
		return
	}
	r.lastCheckpoint = r.countWriter.n
	r.lastCheckpointAt = r.now()
	_ = r.catalog.Checkpoint(context.Background(), r.recordingID, r.countWriter.n, r.lastSeq)
}

func (r *Recorder) rotate() {
	if r.file == nil {
		return
	}
	r.finishFile("complete")
}

func (r *Recorder) finish() {
	r.finishFile("complete")
	r.mu.Lock()
	if r.state == RecRecording {
		r.state = RecIdle
	}
	r.mu.Unlock()
}

func (r *Recorder) finishFile(state string) {
	if r.file == nil {
		return
	}
	if err := r.writer.Close(); err != nil {
		r.fail(err)
		return
	}
	if err := r.file.Sync(); err != nil {
		r.fail(err)
		return
	}
	sum := r.hasher.Sum(nil)
	_ = r.catalog.CloseRecording(context.Background(), r.recordingID, r.countWriter.n, r.lastSeq, hex.EncodeToString(sum), state)
	_ = r.file.Close()
	r.file, r.writer, r.countWriter = nil, nil, nil
	r.fileBytes = 0
}

func (r *Recorder) fail(err error) {
	r.mu.Lock()
	r.state = RecStoppedError
	if strings.Contains(err.Error(), "cap reached") || strings.Contains(err.Error(), "reserve") || strings.Contains(err.Error(), "queue") {
		r.state = RecStoppedLimit
	}
	r.reason = err.Error()
	r.mu.Unlock()
	if r.file != nil {
		_ = r.writer.Close()
		_ = r.file.Close()
		r.file, r.writer, r.countWriter = nil, nil, nil
	}
}

func (r *Recorder) setState(s, reason string) {
	r.mu.Lock()
	r.state, r.reason = s, reason
	r.mu.Unlock()
}

func (r *Recorder) State() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state, r.reason
}

// Stop flushes and closes the current file. It is idempotent.
func (r *Recorder) Stop() {
	select {
	case <-r.stopCh:
		return
	default:
		close(r.stopCh)
	}
	select {
	case <-r.done:
	case <-time.After(10 * time.Second):
	}
}

// StopByUser marks an explicit stop so a new recording does not restart silently.
func (r *Recorder) StopByUser() {
	r.mu.Lock()
	if r.state == RecRecording {
		r.state = RecStoppedUser
		r.reason = "stopped by user"
	}
	r.mu.Unlock()
	r.Stop()
}

// Resume starts a new recording after an explicit user stop.
func (r *Recorder) Resume() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RecRecording {
		return nil
	}
	if !r.cfg.Enabled {
		return errors.New("recording is disabled in configuration")
	}
	if r.state == RecStoppedLimit {
		return errors.New("clear storage or lower the limit before recording again")
	}
	r.stopCh = make(chan struct{})
	r.done = make(chan struct{})
	r.state = RecRecording
	r.reason = ""
	go r.loop()
	return nil
}

func (r *Recorder) WarnFraction() float64 {
	used, err := r.usedBytes()
	if err != nil || r.cfg.CapBytes == 0 {
		return 0
	}
	return float64(used) / float64(r.cfg.CapBytes)
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '/' || r == '\\' || r == '.' || r == 0 {
			out = append(out, '_')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}
