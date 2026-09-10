package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/system"
)

var viewANSIRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripViewANSI(s string) string { return viewANSIRe.ReplaceAllString(s, "") }

func newProcessingCtx() *SharedContext {
	ctx := NewContext(config.Default(), &system.Capabilities{
		GPUName:     "Test GPU",
		BestEncoder: "libx264",
	})
	ctx.Width, ctx.Height = 100, 40
	ctx.Files = []string{"/tmp/clip1.mp4"}
	ctx.FileInfo = []string{"1920x1080 · 24fps · 00:02:15"}
	ctx.Multiplier = 4
	ctx.SourceFPS = 24
	return ctx
}

func feed(p *processingState, evs ...pipeline.Event) {
	for _, ev := range evs {
		p.Update(pipelineEventMsg(ev))
	}
}

func TestProcessing_ProgressEvent(t *testing.T) {
	p := NewProcessingPage(newProcessingCtx()).(*processingState)
	feed(p,
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobAdded},
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobStarted},
		pipeline.Event{JobID: "j1", Type: pipeline.EventStageChange, Stage: pipeline.StageProbing, Message: "1920x1080, 24fps, 00:02:15"},
		pipeline.Event{JobID: "j1", Type: pipeline.EventProgress, Stage: pipeline.StageInterpolating, Progress: 62.5, FPS: 8.4, ETA: 84 * time.Second},
	)

	if p.rifeProgress != 62.5 {
		t.Errorf("rifeProgress = %v, want 62.5", p.rifeProgress)
	}
	if p.rifeStage != "generating frames" {
		t.Errorf("rifeStage = %q", p.rifeStage)
	}
	if p.rifeSpeed != 8.4 {
		t.Errorf("rifeSpeed = %v, want 8.4", p.rifeSpeed)
	}
	if p.sourceWidth != 1920 || p.sourceHeight != 1080 || p.sourceFPS != 24 {
		t.Errorf("source = %dx%d @%v", p.sourceWidth, p.sourceHeight, p.sourceFPS)
	}
	if p.sourceFrames != 3240 {
		t.Errorf("sourceFrames = %d, want 3240", p.sourceFrames)
	}

	feed(p,
		pipeline.Event{JobID: "j1", Type: pipeline.EventStageChange, Stage: pipeline.StageEncoding},
		pipeline.Event{JobID: "j1", Type: pipeline.EventProgress, Stage: pipeline.StageEncoding, Progress: 31.2, ETA: 130 * time.Second},
	)
	if p.encodeProgress != 31.2 {
		t.Errorf("encodeProgress = %v, want 31.2", p.encodeProgress)
	}
	if p.encodeStage != "writing video" {
		t.Errorf("encodeStage = %q", p.encodeStage)
	}
	if len(p.rifeFPSHist) != 1 {
		t.Errorf("fps history = %v, want one sample", p.rifeFPSHist)
	}
}

func TestProcessing_BatchDone(t *testing.T) {
	p := NewProcessingPage(newProcessingCtx()).(*processingState)
	feed(p,
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobAdded},
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobStarted, Time: time.Now()},
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobFailed, Error: errString("probe boom"), Time: time.Now()},
	)

	_, cmd := p.Update(pipelineEventMsg(pipeline.Event{Type: pipeline.EventBatchDone}))
	if cmd == nil {
		t.Fatal("EventBatchDone returned no command")
	}
	msg, ok := cmd().(batchDoneMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want batchDoneMsg", cmd())
	}
	if msg.Summary.Failed != 1 {
		t.Errorf("summary.Failed = %d, want 1", msg.Summary.Failed)
	}
	if len(msg.Summary.Outputs) != 1 || msg.Summary.Outputs[0].Status != "failed" {
		t.Errorf("summary.Outputs = %+v", msg.Summary.Outputs)
	}
}

func TestProcessing_View(t *testing.T) {
	ctx := newProcessingCtx()
	ctx.Files = []string{"/tmp/clip1.mp4", "/tmp/clip2.mp4"}
	p := NewProcessingPage(ctx).(*processingState)
	feed(p,
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobAdded},
		pipeline.Event{JobID: "j2", Type: pipeline.EventJobAdded},
		pipeline.Event{JobID: "j1", Type: pipeline.EventJobStarted},
		pipeline.Event{JobID: "j1", Type: pipeline.EventStageChange, Stage: pipeline.StageProbing, Message: "1920x1080, 24fps, 00:02:15"},
		pipeline.Event{JobID: "j1", Type: pipeline.EventProgress, Stage: pipeline.StageInterpolating, Progress: 50, FPS: 8.4, ETA: 84 * time.Second},
	)

	content := stripViewANSI(p.View().Content)
	for _, want := range []string{
		"👑 vif", "批量进度", "clip1.mp4", "clip2.mp4", "waiting",
		"█", "░", "speed", "eta", "8.4 fps", "50.0%",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("view missing %q in:\n%s", want, content)
		}
	}
}

func TestProcessing_PopupQuit(t *testing.T) {
	p := NewProcessingPage(newProcessingCtx()).(*processingState)

	p.Update(keyPress('q', "q"))
	if !p.popup.Active() {
		t.Fatal("q did not open the quit popup")
	}

	// move to the Quit button and confirm
	p.Update(keyPress(tea.KeyRight, ""))
	_, cmd := p.Update(enterKey())
	if cmd == nil {
		t.Fatal("enter on popup returned no command")
	}
	res, ok := cmd().(popupResultMsg)
	if !ok {
		t.Fatalf("popup cmd returned %T, want popupResultMsg", cmd())
	}
	if res.ID != "quit" || !res.Chosen || res.ButtonIndex != 1 {
		t.Fatalf("popup result = %+v", res)
	}

	_, quitCmd := p.Update(res)
	if quitCmd == nil {
		t.Fatal("confirming quit returned no command")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit cmd returned %T, want tea.QuitMsg", quitCmd())
	}
}

func TestSparkline(t *testing.T) {
	if got := sparkline(nil, 8); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
	if got := sparkline([]float64{5}, 8); got != "▁" {
		t.Errorf("single = %q, want ▁", got)
	}
	got := sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8}, 8)
	if got != "▁▂▃▄▅▆▇█" {
		t.Errorf("ramp = %q", got)
	}
	// width cap keeps only the newest samples
	if got := sparkline([]float64{1, 2, 3, 4}, 2); len([]rune(got)) != 2 {
		t.Errorf("cap = %q, want 2 runes", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
