package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

func (a *API) freeReserveOK() error {
	cfg := a.Config.Config()
	free, err := diskFree(a.dataDir())
	if err != nil {
		return nil
	}
	if free < cfg.Recording.FreeReserveBytes {
		return fmt.Errorf("free space %d is below the %d reserve", free, cfg.Recording.FreeReserveBytes)
	}
	return nil
}

func (a *API) importFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, r, 400, "bad_request", "multipart file field required")
		return
	}
	defer file.Close()
	if hdr.Size > 4<<30 {
		writeErr(w, r, 413, "too_large", "upload exceeds 4 GiB")
		return
	}
	dir := filepath.Join(a.dataDir(), "imports", "uploads")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, r, 500, "io_error", err.Error())
		return
	}
	src := filepath.Join(dir, fmt.Sprintf("upload-%d-%s", time.Now().UnixNano(), filepath.Base(hdr.Filename)))
	out, err := os.OpenFile(src, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		writeErr(w, r, 500, "io_error", err.Error())
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		writeErr(w, r, 500, "io_error", err.Error())
		return
	}
	out.Close()
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{Job: contract.Job{ID: id, Kind: "import", State: "running", DownloadIDs: []string{}, CreatedAtMs: time.Now().UnixMilli()}, Cancel: cancel}
	a.mu.Lock()
	a.jobs[id] = j
	a.mu.Unlock()
	go func() {
		defer cancel()
		res, err := history.ImportF1Bin(ctx, a.Catalog, src, a.dataDir(), a.Reducer.SourceContext(), "import")
		a.mu.Lock()
		defer a.mu.Unlock()
		if err != nil {
			j.Job.State = "failed"
			j.Job.Error = &contract.JobError{Code: "import_failed", Message: err.Error()}
			return
		}
		if ctx.Err() != nil {
			j.Job.State = "cancelled"
			return
		}
		j.Job.State = "succeeded"
		j.Job.ProgressFraction = floatPtr(1)
		j.Job.DownloadIDs = []string{res.RecordingID}
		a.downloads[res.RecordingID] = res.Artifact
	}()
	writeJSON(w, 202, j.Job)
}

func (a *API) exportFile(w http.ResponseWriter, r *http.Request) {
	var req contract.ExportRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	if req.Format != "csv" && req.Format != "jsonl" && req.Format != "parquet" {
		writeErr(w, r, 400, "bad_request", "format must be csv, jsonl or parquet")
		return
	}
	if err := a.freeReserveOK(); err != nil {
		writeErr(w, r, 507, "insufficient_storage", err.Error())
		return
	}
	id := fmt.Sprintf("job-%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{Job: contract.Job{ID: id, Kind: "export", State: "running", DownloadIDs: []string{}, CreatedAtMs: time.Now().UnixMilli()}, Cancel: cancel}
	a.mu.Lock()
	a.jobs[id] = j
	a.mu.Unlock()
	go func() {
		defer cancel()
		path, man, err := a.Engine.Export(ctx, req)
		a.mu.Lock()
		defer a.mu.Unlock()
		if err != nil {
			j.Job.State = "failed"
			j.Job.Error = &contract.JobError{Code: "export_failed", Message: err.Error()}
			return
		}
		if ctx.Err() != nil {
			j.Job.State = "cancelled"
			return
		}
		j.Job.State = "succeeded"
		j.Job.ProgressFraction = floatPtr(1)
		j.Job.DownloadIDs = []string{man.ExportID}
		a.downloads[man.ExportID] = path
	}()
	writeJSON(w, 202, j.Job)
}

func (a *API) getJob(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	j, ok := a.jobs[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeErr(w, r, 404, "not_found", "unknown job")
		return
	}
	writeJSON(w, 200, j.Job)
}

func (a *API) deleteJob(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	j, ok := a.jobs[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeErr(w, r, 404, "not_found", "unknown job")
		return
	}
	if j.Job.State == "running" || j.Job.State == "pending" {
		j.Cancel()
		a.mu.Lock()
		j.Job.State = "cancelled"
		a.mu.Unlock()
	}
	writeJSON(w, 200, j.Job)
}

func (a *API) download(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	path, ok := a.downloads[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeErr(w, r, 404, "not_found", "unknown download")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErr(w, r, 404, "not_found", "artifact missing")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	http.ServeFile(w, r, path)
}

func (a *API) createReplay(w http.ResponseWriter, r *http.Request) {
	var in contract.ReplayCreate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	if in.Speed == 0 {
		in.Speed = 1
	}
	if in.Speed < 0.25 || in.Speed > 8 {
		writeErr(w, r, 400, "bad_request", "speed must be 0.25-8")
		return
	}
	paths, err := a.Catalog.RecordingsForSession(r.Context(), in.SessionID)
	if err != nil || len(paths) == 0 {
		writeErr(w, r, 404, "not_found", "no recording for session")
		return
	}
	src, err := history.LoadReplay(paths[0])
	if err != nil {
		writeErr(w, r, 500, "replay_error", err.Error())
		return
	}
	id := fmt.Sprintf("replay-%d", time.Now().UnixNano())
	rp := history.NewReplay(id, in.SessionID, in.BranchID, in.Speed, in.SkipLongPauses, src)
	a.mu.Lock()
	a.replays[id] = rp
	a.mu.Unlock()
	writeJSON(w, 201, rp.View())
}

func (a *API) patchReplay(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	rp, ok := a.replays[r.PathValue("id")]
	a.mu.Unlock()
	if !ok {
		writeErr(w, r, 404, "not_found", "unknown replay")
		return
	}
	var p contract.ReplayPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	rp.Patch(p)
	writeJSON(w, 200, rp.View())
}

func (a *API) deleteReplay(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	delete(a.replays, r.PathValue("id"))
	a.mu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func floatPtr(v float64) *float64 { return &v }

var _ = errors.New
