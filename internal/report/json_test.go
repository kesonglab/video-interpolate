package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
)

func TestReport_BasicFlow(t *testing.T) {
	r := NewReport(config.Default())

	ev := func(typ pipeline.EventType, id string) pipeline.Event {
		return pipeline.Event{JobID: id, Type: typ, Time: time.Now()}
	}
	r.RecordEvent(ev(pipeline.EventJobStarted, "1"))
	r.RecordEvent(ev(pipeline.EventProgress, "1"))
	r.RecordEvent(pipeline.Event{JobID: "1", Type: pipeline.EventJobCompleted, Message: "/out/a_60fps.mp4 (00:02 → 00:04)", Time: time.Now()})
	r.RecordEvent(pipeline.Event{JobID: "2", Type: pipeline.EventJobFailed, Error: errors.New("rife boom"), Time: time.Now()})
	r.RecordEvent(ev(pipeline.EventJobSkipped, "3"))
	r.RecordEvent(ev(pipeline.EventBatchDone, ""))

	if r.Summary.Total != 3 || r.Summary.Success != 1 || r.Summary.Failed != 1 || r.Summary.Skipped != 1 {
		t.Errorf("summary = %+v, want 1/1/1 of 3", r.Summary)
	}
	if len(r.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(r.Jobs))
	}
	if r.Jobs[0].Status != "success" || r.Jobs[0].Output != "/out/a_60fps.mp4" {
		t.Errorf("job0 = %+v", r.Jobs[0])
	}
	if r.Jobs[1].Status != "failed" || r.Jobs[1].Error != "rife boom" {
		t.Errorf("job1 = %+v", r.Jobs[1])
	}
	if r.Jobs[2].Status != "skipped" {
		t.Errorf("job2 = %+v", r.Jobs[2])
	}

	// enrich pulls input path, duration and size from pipeline jobs
	dir := t.TempDir()
	out := filepath.Join(dir, "a_60fps.mp4")
	if err := writeFile(out, "video"); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-3 * time.Second)
	r.Jobs[0].Output = out
	r.Enrich([]*pipeline.Job{
		{ID: "1", InputPath: "/in/a.mp4", OutputPath: out, StartTime: start, EndTime: time.Now()},
		{ID: "2", InputPath: "/in/b.mp4"},
	})
	if r.Jobs[0].Input != "/in/a.mp4" {
		t.Errorf("input = %q, want /in/a.mp4", r.Jobs[0].Input)
	}
	if r.Jobs[0].DurationMs < 2000 || r.Jobs[0].DurationMs > 4000 {
		t.Errorf("duration_ms = %d, want ~3000", r.Jobs[0].DurationMs)
	}
	if r.Jobs[0].SizeBytes != int64(len("video")) {
		t.Errorf("size = %d, want %d", r.Jobs[0].SizeBytes, len("video"))
	}
	if r.Jobs[1].Input != "/in/b.mp4" {
		t.Errorf("job1 input = %q", r.Jobs[1].Input)
	}
}

func TestReport_Write(t *testing.T) {
	r := NewReport(config.Default())
	r.RecordEvent(pipeline.Event{JobID: "1", Type: pipeline.EventJobCompleted, Message: "/o/x_60fps.mp4 (00:01 → 00:02)"})
	r.Finalize()

	var buf bytes.Buffer
	if err := r.Write(&buf); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version string `json:"version"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
		Jobs []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got.Version != "0.1.0" || got.Summary.Total != 1 {
		t.Errorf("got version=%s total=%d", got.Version, got.Summary.Total)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].Status != "success" || got.Jobs[0].ID != "1" {
		t.Errorf("jobs = %+v", got.Jobs)
	}
}

func TestEventLine(t *testing.T) {
	ev := pipeline.Event{
		JobID:    "7",
		Type:     pipeline.EventJobFailed,
		Stage:    pipeline.StageEncoding,
		Progress: 42,
		FPS:      12.5,
		ETA:      3 * time.Second,
		Error:    errors.New("encode died"),
		Time:     time.Unix(0, 0).UTC(),
	}
	line, err := EventLine(ev)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if got["type"] != "job_failed" || got["stage"] != "encoding" || got["job_id"] != "7" {
		t.Errorf("line = %s", line)
	}
	if got["error"] != "encode died" {
		t.Errorf("error = %v", got["error"])
	}
	if got["progress"] != 42.0 || got["eta_ms"] != float64(3000) {
		t.Errorf("progress/eta = %v/%v", got["progress"], got["eta_ms"])
	}
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
