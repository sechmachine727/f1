package analysis

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"

	"github.com/foxglove/mcap/go/mcap"
	"github.com/parquet-go/parquet-go"
)

type parquetRow struct {
	SchemaVersion    string
	SessionID        string
	BranchID         string
	FragmentID       string
	Seq              string
	ReceivedAtMs     int64
	SessionTimeMs    *int64
	Lap              *int
	CarIndex         *int
	Channel          string
	ValueNumber      *float64
	ValueText        *string
	ValueBoolean     *bool
	Unit             string
	Availability     string
	Basis            string
	SourceID         string
	Confidence       string
	Reason           *string
	RegulationSeason *int
	GameModeCode     *int
	Conditions       string
}

func (e *Engine) Export(ctx context.Context, req contract.ExportRequest) (string, contract.ExportManifest, error) {
	man := contract.ExportManifest{
		SchemaVersion: contract.SchemaVersion, ExportID: newID("exp"), Format: req.Format,
		CreatedAtMs: time.Now().UnixMilli(), SessionID: req.SessionID, BranchID: req.BranchID,
		Range: req.Range, Cars: req.Cars, Channels: req.Channels, Units: map[string]string{},
		Conditions: []string{}, Gaps: []string{}, SourceFiles: []string{},
		DecoderVersion: protocol.Version, MethodVersions: []string{"export-long-form-v1"}, SecretsIncluded: false,
	}
	if len(req.Cars) == 0 {
		req.Cars = []int{0}
	}
	if len(req.Channels) == 0 {
		return "", man, ErrTooManyChannels
	}
	from, _ := strconv.ParseUint(req.Range.FromSeq, 10, 64)
	to, _ := strconv.ParseUint(req.Range.ToSeq, 10, 64)
	ranges, err := e.Catalog.RecordingRanges(ctx, req.SessionID)
	if err != nil {
		return "", man, err
	}
	if len(ranges) == 0 {
		return "", man, history.ErrNotFound
	}
	detail, err := e.Catalog.Session(ctx, req.SessionID)
	if err != nil {
		return "", man, err
	}
	rows := []contract.ExportRow{}
	for _, ch := range req.Channels {
		if s, ok := protocol.Lookup(ch); ok {
			man.Units[ch] = s.Unit
		}
	}
	fragmentFor := func(seq uint64) string {
		for _, r := range ranges {
			if seq >= r.FirstSeq && seq <= r.LastSeq {
				return r.FragmentID
			}
		}
		return detail.Context.FragmentID
	}
	for _, rr := range ranges {
		man.SourceFiles = append(man.SourceFiles, filepath.Base(rr.Path))
		err := scanFile(ctx, rr.Path, func(m *mcap.Message) error {
			seq := uint64(m.Sequence)
			if seq < from || seq > to {
				return nil
			}
			p, err := protocol.Decode(m.Data)
			if err != nil {
				man.Gaps = append(man.Gaps, fmt.Sprintf("seq %d undecodable", seq))
				return nil
			}
			for _, car := range req.Cars {
				for _, ch := range req.Channels {
					spec, ok := protocol.Lookup(ch)
					if !ok || spec.PacketID != p.Header.PacketID {
						continue
					}
					v, ok := extract(p, spec, car)
					if !ok {
						continue
					}
					row := contract.ExportRow{
						SchemaVersion: contract.SchemaVersion, SessionID: req.SessionID, BranchID: req.BranchID,
						FragmentID: fragmentFor(seq), Seq: strconv.FormatUint(seq, 10),
						ReceivedAtMs: int64(m.LogTime / 1e6), SessionTimeMs: int64Ptr(int64(p.Header.SessionTime * 1000)),
						CarIndex: intPtr(car), Channel: ch, Unit: spec.Unit, Availability: "available",
						Basis: "observed", SourceID: "export.raw", Confidence: "high",
						RegulationSeason: detail.Summary.RegulationSeason, GameModeCode: detail.Context.GameModeCode,
						Conditions: conditionsOf(detail.Context),
					}
					switch {
					case spec.Name == "energy.deploy_mode":
						s := protocol.DeployModeLabelForSeason(v, detail.Summary.RegulationSeason)
						row.ValueText = &s
					case spec.Name == "aero.active_mode":
						s := protocol.ActiveAeroLabel(v)
						row.ValueText = &s
					case spec.ValueType == "boolean":
						b := v != 0
						row.ValueBoolean = &b
					default:
						n := v
						row.ValueNumber = &n
					}
					if car != int(p.Header.PlayerCarIndex) && spec.Restricted && v == 0 {
						row.Availability = "restricted"
						row.ValueNumber, row.ValueText, row.ValueBoolean = nil, nil, nil
						r := "sender restricts this channel"
						row.Reason = &r
					}
					rows = append(rows, row)
				}
			}
			return nil
		})
		if err != nil {
			return "", man, err
		}
	}
	man.RowCount = len(rows)
	dir := filepath.Join(e.DataDir, "exports")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", man, err
	}
	zipPath := filepath.Join(dir, man.ExportID+".zip")
	zf, err := os.OpenFile(zipPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", man, err
	}
	zw := zip.NewWriter(zf)
	dw, err := zw.Create("data." + req.Format)
	if err != nil {
		zf.Close()
		return "", man, err
	}
	switch req.Format {
	case "csv":
		man.Conditions = append(man.Conditions, "csv_text_escaping: fields beginning with =, +, -, @ are prefixed with an apostrophe")
		if err := writeCSV(dw, rows); err != nil {
			zf.Close()
			return "", man, err
		}
	case "jsonl":
		enc := json.NewEncoder(dw)
		for _, r := range rows {
			if err := enc.Encode(r); err != nil {
				zf.Close()
				return "", man, err
			}
		}
	case "parquet":
		pw := parquet.NewGenericWriter[parquetRow](dw)
		prs := make([]parquetRow, 0, len(rows))
		for _, r := range rows {
			prs = append(prs, parquetRow{
				SchemaVersion: r.SchemaVersion, SessionID: r.SessionID, BranchID: r.BranchID,
				FragmentID: r.FragmentID, Seq: r.Seq, ReceivedAtMs: r.ReceivedAtMs,
				SessionTimeMs: r.SessionTimeMs, Lap: r.Lap, CarIndex: r.CarIndex, Channel: r.Channel,
				ValueNumber: r.ValueNumber, ValueText: r.ValueText, ValueBoolean: r.ValueBoolean,
				Unit: r.Unit, Availability: r.Availability, Basis: r.Basis, SourceID: r.SourceID,
				Confidence: r.Confidence, Reason: r.Reason, RegulationSeason: r.RegulationSeason,
				GameModeCode: r.GameModeCode, Conditions: r.Conditions,
			})
		}
		if _, err := pw.Write(prs); err != nil {
			zf.Close()
			return "", man, err
		}
		if err := pw.Close(); err != nil {
			zf.Close()
			return "", man, err
		}
	default:
		zf.Close()
		return "", man, fmt.Errorf("unsupported format %q", req.Format)
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		zf.Close()
		return "", man, err
	}
	if err := json.NewEncoder(mw).Encode(man); err != nil {
		zf.Close()
		return "", man, err
	}
	if err := zw.Close(); err != nil {
		zf.Close()
		return "", man, err
	}
	if err := zf.Sync(); err != nil {
		zf.Close()
		return "", man, err
	}
	if err := zf.Close(); err != nil {
		return "", man, err
	}
	return zipPath, man, nil
}

func writeCSV(w interface{ Write([]byte) (int, error) }, rows []contract.ExportRow) error {
	cw := csv.NewWriter(w)
	header := []string{"schema_version", "session_id", "branch_id", "fragment_id", "seq", "received_at_ms", "session_time_ms", "lap", "car_index", "channel", "value_number", "value_text", "value_boolean", "unit", "availability", "basis", "source_id", "confidence", "reason", "regulation_season", "game_mode_code", "conditions"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		rec := []string{r.SchemaVersion, r.SessionID, r.BranchID, r.FragmentID, r.Seq, strconv.FormatInt(r.ReceivedAtMs, 10), optInt64(r.SessionTimeMs), optInt(r.Lap), optInt(r.CarIndex), r.Channel, optFloat(r.ValueNumber), escapeText(r.ValueText), optBool(r.ValueBoolean), r.Unit, r.Availability, r.Basis, r.SourceID, r.Confidence, optStr(r.Reason), optInt(r.RegulationSeason), optInt(r.GameModeCode), r.Conditions}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func escapeText(p *string) string {
	if p == nil {
		return ""
	}
	s := *p
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}

func conditionsOf(c contract.Context) string {
	parts := []string{}
	if c.RegulationSeason != nil {
		parts = append(parts, fmt.Sprintf("season=%d", *c.RegulationSeason))
	}
	if c.TrackID != nil {
		parts = append(parts, fmt.Sprintf("track=%d", *c.TrackID))
	}
	return strings.Join(parts, ";")
}

func optInt(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func optInt64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func optFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}

func optBool(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func optStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
