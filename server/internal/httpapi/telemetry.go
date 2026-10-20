package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"f1tele/server/internal/history"
)

type telemetryLap struct {
	Lap     int     `json:"lap"`
	StartMs int64   `json:"start_ms"`
	LapMs   int64   `json:"lap_ms"`
	Points  [][]any `json:"points"`
}

type telemetryCar struct {
	CarIndex   int            `json:"car_index"`
	Name       string         `json:"name"`
	TeamID     int            `json:"team_id"`
	Restricted bool           `json:"restricted"`
	Laps       []telemetryLap `json:"laps"`
}

const maxTelemetryCars = 4

// sessionTelemetry serves stored per-lap input samples from the catalogue. It
// never scans a recording, so the view stays fast while a session is running.
func (a *API) sessionTelemetry(w http.ResponseWriter, r *http.Request) {
	d, err := a.Catalog.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	branch := r.URL.Query().Get("branch")
	if branch == "" {
		branch = d.SurvivingBranchID
	}
	var cars []int
	if raw := strings.TrimSpace(r.URL.Query().Get("cars")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			n, convErr := strconv.Atoi(strings.TrimSpace(part))
			if convErr != nil || n < 0 {
				writeErr(w, r, 400, "bad_request", "cars must be comma-separated car indices")
				return
			}
			cars = append(cars, n)
		}
	}
	if len(cars) > maxTelemetryCars {
		writeErr(w, r, 400, "too_many_cars", "at most 4 cars per request")
		return
	}
	if len(cars) == 0 {
		cars = []int{intOr(d.Context.PlayerCarIndex, 0)}
	}
	from, _ := strconv.Atoi(r.URL.Query().Get("from_lap"))
	to, _ := strconv.Atoi(r.URL.Query().Get("to_lap"))
	rows, held, err := a.Catalog.SessionLapSamples(r.Context(), d.Summary.ID, branch, cars, from, to)
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	participants, err := a.Catalog.Participants(r.Context(), d.Summary.ID)
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	restricted, err := a.Catalog.SessionRestrictedCars(r.Context(), d.Summary.ID)
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	out := make([]telemetryCar, 0, len(cars))
	for _, car := range cars {
		entry := telemetryCar{CarIndex: car, Name: "Car " + strconv.Itoa(car+1), Restricted: restricted[car], Laps: []telemetryLap{}}
		for _, p := range participants {
			if p.CarIndex == car {
				entry.Name, entry.TeamID = p.Name, p.TeamID
			}
		}
		out = append(out, entry)
	}
	for _, row := range rows {
		for i := range out {
			if out[i].CarIndex == row.CarIndex {
				out[i].Laps = append(out[i].Laps, telemetryLap{Lap: row.Lap, StartMs: row.StartMs, LapMs: row.LapMs, Points: row.Points})
			}
		}
	}
	writeJSON(w, 200, map[string]any{
		"session_id": d.Summary.ID, "branch_id": branch, "regulation_season": d.Summary.RegulationSeason,
		"cars": out, "laps_available": held, "sample_method": history.SampleMethodVersion,
	})
}

func intOr(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}
