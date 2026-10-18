package qualification

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RSSProbe samples the running process resident set size. A sample failure is
// reported as unavailable rather than zero.
type RSSProbe struct {
	mu      sync.Mutex
	Samples []RSSSample
	stop    chan struct{}
}

type RSSSample struct {
	At  time.Time
	RSS int64
	OK  bool
}

func (p *RSSProbe) Start(interval time.Duration) {
	p.stop = make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-p.stop:
				return
			case now := <-t.C:
				rss, err := processRSS()
				p.mu.Lock()
				p.Samples = append(p.Samples, RSSSample{At: now, RSS: rss, OK: err == nil})
				p.mu.Unlock()
			}
		}
	}()
}

func (p *RSSProbe) Stop() { close(p.stop) }

func (p *RSSProbe) Report(path string) (peak, final, growth int64, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.Samples) == 0 {
		return 0, 0, 0, false
	}
	for _, s := range p.Samples {
		if s.OK && s.RSS > peak {
			peak = s.RSS
		}
	}
	last := p.Samples[len(p.Samples)-1]
	if !last.OK {
		return peak, 0, 0, false
	}
	final = last.RSS
	cut := last.At.Add(-90 * time.Minute)
	if p.Samples[0].At.After(cut) {
		// Shorter runs cannot measure 90-minute growth.
		return peak, final, 0, true
	}
	for _, s := range p.Samples {
		if s.OK && !s.At.Before(cut) {
			if final-s.RSS > growth {
				growth = final - s.RSS
			}
		}
	}
	if err := os.MkdirAll(parentDir(path), 0o755); err == nil {
		f, err := os.Create(path)
		if err == nil {
			w := csv.NewWriter(f)
			w.Write([]string{"at", "rss_bytes", "available"})
			for _, s := range p.Samples {
				w.Write([]string{s.At.Format(time.RFC3339Nano), strconv.FormatInt(s.RSS, 10), strconv.FormatBool(s.OK)})
			}
			w.Flush()
			f.Close()
		}
	}
	return peak, final, growth, true
}

func processRSS() (int64, error) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0, err
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}
	return kb * 1024, nil
}

func parentDir(path string) string {
	i := strings.LastIndex(path, "/")
	if i <= 0 {
		return "."
	}
	return path[:i]
}

func fmtBytes(v int64) string { return fmt.Sprintf("%d", v) }
