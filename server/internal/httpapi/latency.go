package httpapi

import (
	"sort"
	"sync"
	"time"
)

// LatencyStats records accepted-observation to delta-enqueue delay. It excludes
// provider/network time and browser render; the qualification report says so.
type LatencyStats struct {
	mu      sync.Mutex
	samples []time.Duration
}

const latencyRing = 200000

func (l *LatencyStats) Observe(d time.Duration) {
	if d < 0 {
		return
	}
	l.mu.Lock()
	if len(l.samples) < latencyRing {
		l.samples = append(l.samples, d)
	} else {
		l.samples[len(l.samples)%latencyRing] = d
	}
	l.mu.Unlock()
}

func (l *LatencyStats) Percentile(p float64) (time.Duration, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.samples) == 0 {
		return 0, 0
	}
	cp := append([]time.Duration(nil), l.samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)-1) * p)
	return cp[idx], len(cp)
}

func (l *LatencyStats) Max() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	var m time.Duration
	for _, d := range l.samples {
		if d > m {
			m = d
		}
	}
	return m
}

func (l *LatencyStats) WriteCSV(path string) error {
	l.mu.Lock()
	cp := append([]time.Duration(nil), l.samples...)
	l.mu.Unlock()
	f, err := createFile(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("latency_ns\n"); err != nil {
		return err
	}
	for _, d := range cp {
		if _, err := f.WriteString(itoa(int64(d)) + "\n"); err != nil {
			return err
		}
	}
	return nil
}
