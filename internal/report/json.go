package report

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
)

// Report is the final machine-readable summary of a run.
type Report struct {
	Version   string         `json:"version"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   time.Time      `json:"ended_at"`
	Config    *config.Config `json:"config"`
	Jobs      []JobResult    `json:"jobs"`
	Summary   Summary        `json:"summary"`
}

// JobResult is one job's outcome.
type JobResult struct {
	ID         string `json:"id"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	Status     string `json:"status"` // success / failed / skipped
	DurationMs int64  `json:"duration_ms"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
	Chunks     int    `json:"chunks,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Summary counts jobs by outcome.
type Summary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

func NewReport(cfg *config.Config) *Report {
	return &Report{
		Version:   "0.1.0",
		StartedAt: time.Now(),
		Config:    cfg,
		Jobs:      []JobResult{},
	}
}

// RecordEvent folds a pipeline event into the report.
func (r *Report) RecordEvent(ev pipeline.Event) {
	switch ev.Type {
	case pipeline.EventJobCompleted:
		r.Jobs = append(r.Jobs, JobResult{
			ID:     ev.JobID,
			Output: outputPathFromMessage(ev.Message),
			Status: "success",
		})
		r.Summary.Success++
		r.Summary.Total++
	case pipeline.EventJobFailed:
		r.Jobs = append(r.Jobs, JobResult{
			ID:     ev.JobID,
			Status: "failed",
			Error:  errString(ev.Error),
		})
		r.Summary.Failed++
		r.Summary.Total++
	case pipeline.EventJobSkipped:
		r.Jobs = append(r.Jobs, JobResult{
			ID:     ev.JobID,
			Status: "skipped",
		})
		r.Summary.Skipped++
		r.Summary.Total++
	}
}

// Enrich fills fields events don't carry (input path, timing, output size).
func (r *Report) Enrich(jobs []*pipeline.Job) {
	byID := make(map[string]*pipeline.Job, len(jobs))
	for _, j := range jobs {
		byID[j.ID] = j
	}
	for i := range r.Jobs {
		j := byID[r.Jobs[i].ID]
		if j == nil {
			continue
		}
		r.Jobs[i].Input = j.InputPath
		if r.Jobs[i].Output == "" {
			r.Jobs[i].Output = j.OutputPath
		}
		if !j.StartTime.IsZero() && !j.EndTime.IsZero() {
			r.Jobs[i].DurationMs = j.EndTime.Sub(j.StartTime).Milliseconds()
		}
		if r.Jobs[i].Status == "success" && r.Jobs[i].Output != "" {
			if fi, err := os.Stat(r.Jobs[i].Output); err == nil {
				r.Jobs[i].SizeBytes = fi.Size()
			}
		}
	}
}

// Finalize stamps the end time.
func (r *Report) Finalize() {
	r.EndedAt = time.Now()
}

// Write serializes the report as pretty JSON.
func (r *Report) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// EventLine serializes a single pipeline event as one NDJSON line.
func EventLine(ev pipeline.Event) ([]byte, error) {
	return json.Marshal(map[string]any{
		"job_id":   ev.JobID,
		"type":     ev.Type.String(),
		"stage":    string(ev.Stage),
		"progress": ev.Progress,
		"fps":      ev.FPS,
		"eta_ms":   ev.ETA.Milliseconds(),
		"message":  ev.Message,
		"error":    errString(ev.Error),
		"time":     ev.Time.Format(time.RFC3339Nano),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// outputPathFromMessage strips the trailing " (dur → dur)" off completion messages.
func outputPathFromMessage(msg string) string {
	if i := strings.Index(msg, " ("); i > 0 {
		return msg[:i]
	}
	return msg
}
