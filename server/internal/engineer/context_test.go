package engineer

import (
	"fmt"
	"testing"

	"f1tele/server/internal/contract"
)

func TestContextBounds(t *testing.T) {
	p := 3
	obs := contract.Observation{SchemaVersion: contract.SchemaVersion, Context: contract.Context{PlayerCarIndex: &p, SeasonEvidence: []string{}}}
	for i := 0; i < 1200; i++ {
		car := i % 24
		obs.Metrics = append(obs.Metrics, contract.Metric{ID: fmt.Sprintf("m%d", i), Channel: "car.speed_mps", CarIndex: &car,
			Measurement: contract.Available(1.0, "m/s", "t")})
	}
	for i := 0; i < 300; i++ {
		sev := "routine"
		if i%50 == 0 {
			sev = "critical"
		}
		obs.Events = append(obs.Events, contract.Event{ID: fmt.Sprintf("e%d", i), Seq: fmt.Sprint(i), Type: "other", Severity: sev, EvidenceIDs: []string{}})
	}
	bounded := BoundObservation(obs, 3)
	if len(bounded.Metrics) != maxMetrics {
		t.Fatalf("metrics = %d", len(bounded.Metrics))
	}
	if !bounded.Coverage.Truncated || bounded.Coverage.OmittedEventCount <= 0 {
		t.Fatalf("coverage %#v", bounded.Coverage)
	}
	if len(bounded.Events) > maxRecentEvents+maxCritical {
		t.Fatalf("events = %d", len(bounded.Events))
	}
	criticals := 0
	for _, e := range bounded.Events {
		if e.Severity == "critical" {
			criticals++
		}
	}
	if criticals != 6 {
		t.Fatalf("critical events kept = %d, want all 6", criticals)
	}
	// Player-car metrics rank ahead of rivals.
	first := bounded.Metrics[0]
	if first.CarIndex == nil || *first.CarIndex != 3 {
		t.Fatalf("first metric is not the player's: %#v", first)
	}
	// A tiny prompt fits any sane endpoint; an oversized one fails explicitly.
	if err := CheckContextFit(1000, 16384); err != nil {
		t.Fatal(err)
	}
	if err := CheckContextFit(20000, 16384); err == nil {
		t.Fatal("oversized prompt accepted")
	}
}

func TestBoundObservationKeepsSmallInputs(t *testing.T) {
	obs := contract.Observation{SchemaVersion: contract.SchemaVersion, Context: contract.Context{SeasonEvidence: []string{}},
		Metrics: []contract.Metric{{ID: "a", Channel: "car.speed_mps", Measurement: contract.Available(1.0, "m/s", "t")}},
		Events:  []contract.Event{{ID: "e", Seq: "1", Type: "other", Severity: "routine", EvidenceIDs: []string{}}}}
	bounded := BoundObservation(obs, 0)
	if bounded.Coverage.Truncated || len(bounded.Metrics) != 1 || len(bounded.Events) != 1 {
		t.Fatalf("small input changed: %#v", bounded)
	}
}
