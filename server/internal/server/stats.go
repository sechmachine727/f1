package server

import (
	"crypto/sha256"
	"hash"
	"sync"
)

// IntakeStats is the receiver-side oracle: it counts and digests datagrams in
// arrival order, independently of the recorder and of live frame filtering.
type IntakeStats struct {
	mu       sync.Mutex
	received uint64
	bytes    uint64
	digest   hash.Hash
	done     bool
	sum      []byte
}

func (s *IntakeStats) Observe(n int, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.digest == nil {
		s.digest = sha256.New()
	}
	s.received++
	s.bytes += uint64(n)
	s.digest.Write(data)
}

// Snapshot freezes the digest so repeated reports are stable.
func (s *IntakeStats) Snapshot() (uint64, uint64, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.digest == nil {
		return 0, 0, nil
	}
	if !s.done {
		s.sum = s.digest.Sum(nil)
		s.done = true
	}
	return s.received, s.bytes, s.sum
}

// ObserveReopen continues the digest; used only between test phases.
func (s *IntakeStats) Reopen() {
	s.mu.Lock()
	s.done = false
	s.mu.Unlock()
}
