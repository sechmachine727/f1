package analysis

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"

	"github.com/parquet-go/parquet-go"
)

func fixture(t *testing.T) (*Engine, string, string) {
	t.Helper()
	dir := t.TempDir()
	cat, err := history.OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	cx := contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}}
	_ = cat.CreateSession(ctx, "s", cx)
	_ = cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	rec := history.NewRecorder(contract.RecordingConfig{Enabled: true, CapBytes: 1 << 30, FreeReserveBytes: 0, WarnFraction: 0.8, QueueBytes: 1 << 20, ChunkBytes: 1 << 16, ChunkFlushMs: 50, RotateBytes: 1 << 30}, cat, dir, nil)
	rec.Start()
	rec.SetIdentity("s", "f", "c")
	for i := 0; i < 240; i++ {
		pid := uint8(2)
		if i%2 == 1 {
			pid = 6
		}
		cars := map[int]map[string]float64{0: {"m_lapDistance": float64(i) * 20, "m_speed": 200, "m_currentLapNum": 1}}
		data := protocol.BuildPacket(2026, pid, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, SessionTime: float64(i) * 0.1, OverallFrameID: uint32(i)}, nil, cars, nil)
		rec.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)), data)
	}
	rec.Stop()
	laps := []contract.LapSummary{
		{ID: "l1", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, StartSeq: "1", EndSeq: "120", LapTimeMs: contract.Available(90000, "ms", "t"), DistanceM: contract.Unavailable("not_received", "m", "t", "x"), Compound: contract.Available("C3", "compound", "t"), Valid: true, Complete: true, Conditions: "dry", Gaps: []string{}},
		{ID: "l2", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 2, StartSeq: "121", EndSeq: "240", LapTimeMs: contract.Available(90500, "ms", "t"), DistanceM: contract.Unavailable("not_received", "m", "t", "x"), Compound: contract.Available("C3", "compound", "t"), Valid: true, Complete: true, Conditions: "dry", Gaps: []string{}},
	}
	for _, l := range laps {
		if err := cat.InsertLap(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	return &Engine{Catalog: cat, DataDir: dir}, dir, "s"
}

func TestQueryBoundsAndGaps(t *testing.T) {
	e, _, sid := fixture(t)
	if _, err := e.Query(context.Background(), contract.Query{SessionID: sid, BranchID: "b", Channels: make([]string, 17), MaxPoints: 10, ToSessionTimeMs: 1}); err != ErrTooManyChannels {
		t.Fatalf("channel limit: %v", err)
	}
	if _, err := e.Query(context.Background(), contract.Query{SessionID: sid, BranchID: "b", Channels: []string{"car.speed_mps"}, MaxPoints: 20001, ToSessionTimeMs: 1}); err != ErrTooManyPoints {
		t.Fatalf("point limit: %v", err)
	}
	res, err := e.Query(context.Background(), contract.Query{SessionID: sid, BranchID: "b", CarIndices: []int{0}, Channels: []string{"car.speed_mps"}, FromSessionTimeMs: 0, ToSessionTimeMs: 100000, MaxPoints: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Series) != 1 || len(res.Series[0].Points) == 0 || len(res.Series[0].Points) > 50 {
		t.Fatalf("series %#v", res.Series)
	}
}

func TestCompareKeepsInvalidMarkers(t *testing.T) {
	e, _, sid := fixture(t)
	res, err := e.Compare(context.Background(), contract.CompareQuery{SessionID: sid, BranchID: "b", LapIDs: []string{"l1", "l2"}, Channels: []string{"car.speed_mps"}, MaxPoints: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Comparable {
		t.Fatalf("identical conditions marked different: %#v", res.Differences)
	}
	if len(res.Series) != 2 || len(res.Series[0].Points) != 201 {
		t.Fatalf("aligned series %d/%d", len(res.Series), len(res.Series[0].Points))
	}
	// A lap without distance cannot be aligned and must leave gaps, not a trace.
	gapped := 0
	for _, p := range res.Series[0].Points {
		if p.Measurement.Availability != "available" {
			gapped++
		}
	}
	if gapped == 0 {
		t.Fatal("expected gap markers outside the distance range")
	}
}

func TestExportRoundTripAndEscaping(t *testing.T) {
	e, dir, sid := fixture(t)
	req := contract.ExportRequest{SessionID: sid, BranchID: "b", Range: contract.Range{FromSeq: "1", ToSeq: "240"}, Cars: []int{0}, Channels: []string{"car.speed_mps"}, Format: "csv"}
	path, man, err := e.Export(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if man.RowCount == 0 || man.SecretsIncluded {
		t.Fatalf("manifest %#v", man)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var dataFile, manifestFile bool
	for _, f := range zr.File {
		switch f.Name {
		case "data.csv":
			dataFile = true
			rc, _ := f.Open()
			rows, err := csv.NewReader(rc).ReadAll()
			rc.Close()
			if err != nil || len(rows) != man.RowCount+1 {
				t.Fatalf("csv rows %d want %d err %v", len(rows), man.RowCount+1, err)
			}
		case "manifest.json":
			manifestFile = true
			rc, _ := f.Open()
			var parsed contract.ExportManifest
			if err := json.NewDecoder(rc).Decode(&parsed); err != nil {
				t.Fatal(err)
			}
			rc.Close()
			if parsed.ExportID != man.ExportID {
				t.Fatal("manifest mismatch")
			}
		}
	}
	if !dataFile || !manifestFile {
		t.Fatal("zip must contain exactly data.csv and manifest.json")
	}
	// JSONL preserves native nulls and numbers.
	req.Format = "jsonl"
	p2, man2, err := e.Export(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	zr2, _ := zip.OpenReader(p2)
	defer zr2.Close()
	for _, f := range zr2.File {
		if f.Name == "data.jsonl" {
			rc, _ := f.Open()
			line, _ := io.ReadAll(rc)
			rc.Close()
			var row contract.ExportRow
			if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(line)), "\n")[0]), &row); err != nil {
				t.Fatal(err)
			}
			if row.Availability == "restricted" && row.ValueNumber != nil {
				t.Fatal("restricted row carried a value")
			}
		}
	}
	if man2.Format != "jsonl" {
		t.Fatal("format not recorded")
	}
	// Parquet must reopen with typed nullable columns.
	req.Format = "parquet"
	p3, man3, err := e.Export(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	zr3, err := zip.OpenReader(p3)
	if err != nil {
		t.Fatal(err)
	}
	defer zr3.Close()
	for _, f := range zr3.File {
		if f.Name != "data.parquet" {
			continue
		}
		rc, _ := f.Open()
		raw, _ := io.ReadAll(rc)
		rc.Close()
		rows, err := parquet.Read[parquetRow](bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatalf("parquet reopen: %v", err)
		}
		if int64(len(rows)) != int64(man3.RowCount) {
			t.Fatalf("parquet rows %d want %d", len(rows), man3.RowCount)
		}
		if len(rows) == 0 || rows[0].Channel == "" {
			t.Fatal("parquet row lost its channel")
		}
	}
	_ = dir
	_ = os.Remove(path)
}

func TestCSVEscapesFormulas(t *testing.T) {
	s := "=cmd|' /C calc'!A0"
	if got := escapeText(&s); !strings.HasPrefix(got, "'=") {
		t.Fatalf("not escaped: %q", got)
	}
	n := 1.5
	if optFloat(&n) != "1.5" {
		t.Fatal("numeric sign/format changed")
	}
}
