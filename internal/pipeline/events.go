package pipeline

// Event types and Stage constants live in orchestrator.go.

func (t EventType) String() string {
	switch t {
	case EventJobAdded:
		return "job_added"
	case EventJobStarted:
		return "job_started"
	case EventStageChange:
		return "stage_change"
	case EventProgress:
		return "progress"
	case EventJobCompleted:
		return "job_completed"
	case EventJobFailed:
		return "job_failed"
	case EventJobSkipped:
		return "job_skipped"
	case EventBatchDone:
		return "batch_done"
	}
	return "unknown"
}
