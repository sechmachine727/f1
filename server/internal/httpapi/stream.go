package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/state"

	"github.com/coder/websocket"
)

const (
	maxIncoming = 64 << 10
	sendQueue   = 256
)

type Hub struct {
	mu      sync.Mutex
	clients map[*streamClient]bool
	alerts  []contract.Alert
}

type fingerprint struct {
	value any
	avail string
	asOf  int64
	unit  string
}

type streamClient struct {
	mu      sync.Mutex
	sub     contract.StreamSubscription
	out     chan contract.StreamMessage
	last    map[string]fingerprint
	seq     uint64
	lastEvt string
}

func printOf(m contract.Measurement) fingerprint {
	var asOf int64
	if m.AsOfMs != nil {
		asOf = *m.AsOfMs
	}
	return fingerprint{value: m.Value, avail: m.Availability, asOf: asOf, unit: m.Unit}
}

func NewHub() *Hub { return &Hub{clients: map[*streamClient]bool{}} }

func (h *Hub) Broadcast(msgs ...contract.StreamMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		for _, m := range msgs {
			select {
			case c.out <- m:
			default:
				// Bounded queue: force a fresh snapshot rather than growing memory.
				select {
				case <-c.out:
				default:
				}
				c.out <- contract.StreamMessage{SchemaVersion: contract.SchemaVersion, Type: "resync_required", SourceID: "live", Seq: "0", BaseSeq: nil, Metrics: []contract.Metric{}, RemovedMetricIDs: []string{}, Events: []contract.Event{}, EngineerOutputs: []contract.AgentOutput{}, Alerts: []contract.Alert{}, Jobs: []contract.Job{}}
			}
		}
	}
}

func (a *API) stream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(maxIncoming)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &streamClient{last: map[string]fingerprint{}, sub: defaultSubscription(a), out: make(chan contract.StreamMessage, sendQueue)}
	if a.Hub == nil {
		a.Hub = NewHub()
	}
	a.Hub.mu.Lock()
	a.Hub.clients[c] = true
	a.Hub.mu.Unlock()
	defer func() {
		a.Hub.mu.Lock()
		delete(a.Hub.clients, c)
		a.Hub.mu.Unlock()
	}()

	go func() {
		defer cancel()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var sub contract.StreamSubscription
			if err := json.Unmarshal(data, &sub); err != nil || sub.Type != "subscribe" {
				continue
			}
			if sub.MaxHz < 1 || sub.MaxHz > 60 {
				sub.MaxHz = 20
			}
			if sub.SourceID == "" {
				sub.SourceID = "live"
			}
			c.mu.Lock()
			c.sub = sub
			// Force a fresh snapshot for the new subscription.
			c.last = map[string]fingerprint{}
			c.lastEvt = ""
			c.mu.Unlock()
			select {
			case c.out <- a.snapshotMessage(c):
			default:
			}
		}
	}()

	if err := conn.Write(ctx, websocket.MessageText, mustJSON(a.snapshotMessage(c))); err != nil {
		return
	}
	c.mu.Lock()
	hz := c.sub.MaxHz
	c.mu.Unlock()
	ticker := time.NewTicker(time.Second / time.Duration(hz))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-c.out:
			if err := conn.Write(ctx, websocket.MessageText, mustJSON(m)); err != nil {
				return
			}
		case <-ticker.C:
			if next := a.deltaMessage(c); next != nil {
				if err := conn.Write(ctx, websocket.MessageText, mustJSON(*next)); err != nil {
					return
				}
			}
		}
	}
}

func defaultSubscription(a *API) contract.StreamSubscription {
	// The pit wall compares the whole field, so one socket carries every car.
	// Deltas only contain changed metrics, so idle cars cost nothing per tick.
	all := make([]int, 24)
	for i := range all {
		all[i] = i
	}
	return contract.StreamSubscription{Type: "subscribe", SchemaVer: contract.SchemaVersion, RequestID: "initial", SourceID: "live", CarIndices: all, Channels: []string{}, MaxHz: 20}
}

func (a *API) snapshotMessage(c *streamClient) contract.StreamMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	cars := c.sub.CarIndices
	if len(cars) == 0 {
		cars = []int{a.Reducer.PlayerCar()}
	}
	snap := a.Reducer.Snapshot(cars, c.sub.Channels)
	ctx := snap.Context
	c.seq++
	seq := strconv.FormatUint(c.seq, 10)
	c.last = map[string]fingerprint{}
	for _, m := range snap.Metrics {
		c.last[m.ID] = printOf(m.Measurement)
	}
	evts := a.eventsFor(c, snap.Events)
	return contract.StreamMessage{
		SchemaVersion: contract.SchemaVersion, Type: "snapshot", SourceID: "live", Seq: seq, BaseSeq: nil,
		Context: &ctx, Metrics: snap.Metrics, RemovedMetricIDs: []string{}, Events: evts,
		Status: &snap.RecordingState, EngineerOutputs: []contract.AgentOutput{}, Alerts: a.Arbiter.Alerts(), Jobs: []contract.Job{},
	}
}

func (a *API) deltaMessage(c *streamClient) *contract.StreamMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	cars := c.sub.CarIndices
	if len(cars) == 0 {
		cars = []int{a.Reducer.PlayerCar()}
	}
	snap := a.Reducer.Snapshot(cars, c.sub.Channels)
	if !snap.LastAcceptedAt.IsZero() && c.sub.MaxHz >= 20 {
		a.Latency.Observe(time.Since(snap.LastAcceptedAt))
	}
	var changed []contract.Metric
	var removed []string
	for _, m := range snap.Metrics {
		fp := printOf(m.Measurement)
		if c.last[m.ID] != fp {
			c.last[m.ID] = fp
			changed = append(changed, m)
		}
	}
	// If a metric disappears entirely (channel set changed), delete it.
	present := map[string]bool{}
	for _, m := range snap.Metrics {
		present[m.ID] = true
	}
	for id := range c.last {
		if !present[id] {
			delete(c.last, id)
			removed = append(removed, id)
		}
	}
	evts := a.eventsFor(c, snap.Events)
	if len(changed) == 0 && len(removed) == 0 && len(evts) == 0 {
		return nil
	}
	prev := strconv.FormatUint(c.seq, 10)
	c.seq++
	seq := strconv.FormatUint(c.seq, 10)
	ctx := snap.Context
	var status *string
	st := snap.RecordingState
	status = &st
	return &contract.StreamMessage{
		SchemaVersion: contract.SchemaVersion, Type: "delta", SourceID: "live", Seq: seq, BaseSeq: &prev,
		Context: &ctx, Metrics: changed, RemovedMetricIDs: removed, Events: evts, Status: status,
		EngineerOutputs: []contract.AgentOutput{}, Alerts: []contract.Alert{}, Jobs: []contract.Job{},
	}
}

func (a *API) eventsFor(c *streamClient, all []contract.Event) []contract.Event {
	var out []contract.Event
	for _, e := range all {
		if e.ID == c.lastEvt {
			continue
		}
		out = append(out, e)
	}
	if len(out) > 0 {
		c.lastEvt = out[len(out)-1].ID
	}
	if out == nil {
		out = []contract.Event{}
	}
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func channelCatalog() []contract.Channel { return protocol.Catalog() }

var _ = state.Snapshot{}
var _ = fmt.Sprint
