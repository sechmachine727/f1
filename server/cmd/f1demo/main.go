// Command f1demo sends the qualification corpus to a local server so the
// dashboard can be exercised without the game. It is a development helper; the
// rated workload still runs through server/qualification.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"f1tele/server/internal/protocol"
	"f1tele/server/qualification"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:20777", "UDP target")
	seconds := flag.Int("seconds", 0, "stop after N seconds (0 runs until interrupted)")
	rate := flag.Int("rate", 2048, "datagrams per second")
	seed := flag.Int64("seed", 20261018, "workload seed")
	flag.Parse()

	conn, err := net.Dial("udp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	corpus := &qualification.Corpus{Seed: *seed, SessionUID: uint64(*seed), Player: 0}

	start := time.Now()
	deadline := time.Time{}
	if *seconds > 0 {
		deadline = start.Add(time.Duration(*seconds) * time.Second)
	}
	for slot := int64(0); deadline.IsZero() || time.Now().Before(deadline); slot++ {
		second := int(slot / int64(*rate))
		within := int(slot % int64(*rate))
		data := corpus.BuildSlot(second, within)
		if _, err := conn.Write(data); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			return
		}
		target := start.Add(time.Duration(float64(slot+1) / float64(*rate) * float64(time.Second)))
		if d := time.Until(target); d > 0 {
			time.Sleep(d)
		}
	}
	_ = protocol.Version
}
