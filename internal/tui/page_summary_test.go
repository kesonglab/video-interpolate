package tui

import (
	"strings"
	"testing"
	"time"
)

func TestSummary_View(t *testing.T) {
	ctx := newProcessingCtx()
	ctx.Summary = BatchSummary{
		Success: 2,
		Failed:  1,
		Elapsed: 14*time.Minute + 32*time.Second,
		Outputs: []JobOutput{
			{Path: "/tmp/clip1_96fps.mp4", Size: 312 << 20, Duration: 3*time.Minute + 12*time.Second, Status: "done"},
			{Path: "/tmp/clip2_96fps.mp4", Size: 1200 << 20, Duration: 8*time.Minute + 45*time.Second, Status: "done"},
			{Path: "/tmp/clip3_96fps.mp4", Status: "failed", Error: "encoder timeout"},
		},
	}
	p := NewSummaryPage(ctx).(*summaryState)

	content := p.View().Content
	for _, want := range []string{"Download Results", "Success", "Failed", "clip1_96fps.mp4", "clip2_96fps.mp4", "encoder timeout", "312"} {
		if !strings.Contains(content, want) {
			t.Errorf("summary view missing %q in:\n%s", want, stripViewANSI(content))
		}
	}
}

func TestSummary_Restart(t *testing.T) {
	ctx := newProcessingCtx()
	p := NewSummaryPage(ctx).(*summaryState)

	_, cmd := p.Update(keyPress('r', "r"))
	if cmd == nil {
		t.Fatal("r returned no command")
	}
	if _, ok := cmd().(resetMsg); !ok {
		t.Fatalf("r returned %T, want resetMsg", cmd())
	}
}
