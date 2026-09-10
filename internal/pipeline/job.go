package pipeline

import (
	"sync"
	"time"
)

type JobState int

const (
	JobPending JobState = iota
	JobProbing
	JobDecoding
	JobInterpolating
	JobEncoding
	JobDone
	JobFailed
	JobSkipped
)

type Job struct {
	ID         string
	InputPath  string
	OutputPath string
	State      JobState
	Progress   float64
	Stage      Stage
	FPS        float64
	ETA        time.Duration
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	mu         sync.Mutex
}

func (j *Job) SetState(s JobState) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.State = s
}

func (j *Job) SetStage(s Stage) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Stage = s
}

func (j *Job) SetProgress(p float64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress = p
}

func (j *Job) SetFPS(fps float64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.FPS = fps
}

func (j *Job) SetETA(eta time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.ETA = eta
}

func (j *Job) SetError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Error = err
}

// Terminal reports whether the job is in a finished state (done/failed/skipped).
func (j *Job) Terminal() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.State == JobDone || j.State == JobFailed || j.State == JobSkipped
}

// Snapshot returns a copy safe for TUI rendering.
func (j *Job) Snapshot() Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return Job{
		ID:         j.ID,
		InputPath:  j.InputPath,
		OutputPath: j.OutputPath,
		State:      j.State,
		Progress:   j.Progress,
		Stage:      j.Stage,
		FPS:        j.FPS,
		ETA:        j.ETA,
		Error:      j.Error,
		StartTime:  j.StartTime,
		EndTime:    j.EndTime,
	}
}
