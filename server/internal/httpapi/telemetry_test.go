package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

func telemetryFixture(t *testing.T) Deps {
	t.Helper()
	d, _ := deps(t)
	ctx := context.Background()
	if err := d.Catalog.EnsureSourceContext(ctx, "src", "ip"); err != nil {
		t.Fatal(err)
	}
	player := 0
	cx := contract.Context{SourceContextID: "src", SessionID: "s", BranchID: "b", PlayerCarIndex: &player, ModeFamily: "race"}
	if err := d.Catalog.CreateSession(ctx, "s", cx); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	for car, name := range map[int]string{0: "LECLERC", 1: "HAMILTON"} {
		if err := d.Catalog.UpsertParticipant(ctx, "s", car, name, car+1, 16, false, car == 0, 1); err != nil {
			t.Fatal(err)
		}
		if err := d.Catalog.UpsertLapSamples(ctx, "s", "b", history.LapSamples{
			CarIndex: car, Lap: 1, StartMs: 1000, LapMs: 90000,
			Points: [][]any{{int64(1000), 0.5, 0.0, float64(6), float64(1000000), float64(2), true, false}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestSessionTelemetryRoute(t *testing.T) {
	d := telemetryFixture(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/sessions/s/telemetry?cars=0,1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body struct {
		SessionID     string         `json:"session_id"`
		Cars          []telemetryCar `json:"cars"`
		LapsAvailable int            `json:"laps_available"`
		SampleMethod  string         `json:"sample_method"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SessionID != "s" || len(body.Cars) != 2 || body.LapsAvailable != 2 {
		t.Fatalf("body %#v", body)
	}
	if body.SampleMethod != history.SampleMethodVersion {
		t.Fatalf("method %q", body.SampleMethod)
	}
	if body.Cars[0].Name != "LECLERC" || len(body.Cars[0].Laps) != 1 || len(body.Cars[0].Laps[0].Points) != 1 {
		t.Fatalf("car %#v", body.Cars[0])
	}
	if !body.Cars[1].Restricted {
		t.Fatalf("restricted flag %#v", body.Cars[1])
	}
}

func TestSessionTelemetryBounds(t *testing.T) {
	d := telemetryFixture(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	for path, want := range map[string]int{
		"/api/v1/sessions/nope/telemetry":                404,
		"/api/v1/sessions/s/telemetry?cars=0,1,2,3,4":    400,
		"/api/v1/sessions/s/telemetry?cars=x":            400,
		"/api/v1/sessions/s/telemetry":                   200,
		"/api/v1/sessions/s/telemetry?cars=0&from_lap=5": 200,
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s status %d want %d", path, resp.StatusCode, want)
		}
	}
}

func TestSessionTelemetryWithoutSamples(t *testing.T) {
	d, _ := deps(t)
	ctx := context.Background()
	if err := d.Catalog.EnsureSourceContext(ctx, "src", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateSession(ctx, "empty", contract.Context{SourceContextID: "src", SessionID: "empty", BranchID: "b", ModeFamily: "race"}); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "empty", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/sessions/empty/telemetry?cars=0")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body struct {
		Cars          []telemetryCar `json:"cars"`
		LapsAvailable int            `json:"laps_available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.LapsAvailable != 0 || len(body.Cars) != 1 || len(body.Cars[0].Laps) != 0 {
		t.Fatalf("empty body %#v", body)
	}
}
