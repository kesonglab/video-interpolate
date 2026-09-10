package tui

import (
	"time"

	"github.com/kesonglab/video-interpolate/internal/pipeline"
)

// pipelineEventMsg wraps a pipeline event for bubbletea Update.
type pipelineEventMsg pipeline.Event

// startProcessingMsg triggers the actual pipeline start (emitted by Confirm).
type startProcessingMsg struct{}

// batchDoneMsg emitted when the pipeline signals EventBatchDone.
type batchDoneMsg struct {
	Summary BatchSummary
}

// BatchSummary is the end-of-run rollup shown on the Summary screen.
type BatchSummary struct {
	Success int
	Failed  int
	Skipped int
	Elapsed time.Duration
	Outputs []JobOutput
}

// JobOutput is one finished job's result line.
type JobOutput struct {
	Path     string
	Size     int64
	Duration time.Duration
	Status   string // "success" / "failed" / "skipped"
	Error    string
}

// popupResultMsg returned by popup modal.
type popupResultMsg struct {
	ID          string // "quit" / "cancel" / "skip"
	Chosen      bool
	ButtonIndex int
}

// systemStatsTickMsg fires every 2 seconds for system stats refresh.
type systemStatsTickMsg time.Time

// systemStatsMsg carries a fresh stats sample back to the UI.
type systemStatsMsg struct {
	CPU float64
	Mem float64
	GPU float64
}
