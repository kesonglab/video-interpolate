package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/system"
)

// TestTUIEndToEnd drives Welcome → Files → Multiplier → Encoder → Confirm →
// Processing → Summary with real pipeline events. The source file doesn't
// exist, so the job fails fast at probe time and the batch still completes.
func TestTUIEndToEnd(t *testing.T) {
	cfg := config.Default()
	cfg.OutputDir = t.TempDir()
	a := New(cfg, &system.Capabilities{GPUName: "Test GPU", BestEncoder: "libx264"})
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	src := filepath.Join(t.TempDir(), "clip1.mp4")

	pumpNav(a, enterKey()) // welcome → files
	if a.page != pageFiles {
		t.Fatalf("page = %d, want files", a.page)
	}

	fp := a.pages[pageFiles].(*filePickerPage)
	fp.files = []addedFile{{Path: src, Info: "1920x1080 · 24fps · 00:02:15", Found: true}}

	pumpNav(a, enterKey()) // files → multiplier
	if a.page != pageMultiplier {
		t.Fatalf("page = %d, want multiplier", a.page)
	}
	pumpNav(a, enterKey()) // multiplier → encoder
	if a.page != pageEncoder {
		t.Fatalf("page = %d, want encoder", a.page)
	}
	pumpNav(a, enterKey()) // encoder → confirm
	if a.page != pageConfirm {
		t.Fatalf("page = %d, want confirm", a.page)
	}

	_, cmd := a.Update(enterKey())
	if cmd == nil {
		t.Fatal("confirm returned no command")
	}
	start, ok := cmd().(startProcessingMsg)
	if !ok {
		t.Fatalf("confirm cmd = %T, want startProcessingMsg", cmd())
	}
	a.Update(start)
	if a.page != pageProcessing {
		t.Fatalf("page = %d, want processing", a.page)
	}
	if a.ctx.EventsCh == nil {
		t.Fatal("pipeline event channel not set")
	}

	drainUntilSummary(t, a)
	if a.page != pageSummary {
		t.Fatalf("page = %d, want summary", a.page)
	}
	if a.ctx.Summary.Failed != 1 {
		t.Fatalf("summary = %+v, want one failed job", a.ctx.Summary)
	}
}

// drainUntilSummary feeds buffered pipeline events into the app until it lands
// on the summary page.
func drainUntilSummary(t *testing.T, a *App) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for a.page != pageSummary {
		select {
		case ev, ok := <-a.ctx.EventsCh:
			if !ok {
				a.Update(batchDoneMsg{})
				return
			}
			if ev.Type == pipeline.EventBatchDone {
				_, cmd := a.Update(pipelineEventMsg(ev))
				if cmd != nil {
					if msg := cmd(); msg != nil {
						a.Update(msg)
					}
				}
				return
			}
			a.Update(pipelineEventMsg(ev))
		case <-deadline:
			t.Fatalf("timed out on page %d waiting for summary", a.page)
		}
	}
}
