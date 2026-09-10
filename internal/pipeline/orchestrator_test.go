package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/encode"
	"github.com/kesonglab/video-interpolate/internal/interpolate"
	"github.com/kesonglab/video-interpolate/internal/probe"
	"github.com/kesonglab/video-interpolate/internal/system"
)

// mockEnv stands in for probe/decode/interpolate/encode.
type mockEnv struct {
	probeInfo    *probe.VideoInfo
	probeErr     error
	decodeN      int
	decodeErr    error
	decodeCalls  int
	decodeOpts   []encode.DecodeOpts
	interpErr    error
	interpErrOn  int // fail on this 1-based interp call (0 = never)
	interpCalls  int
	encodeErr    error
	encodeErrOn  int // fail on this 1-based encode call (0 = never)
	encodeCalls  int
	encodeOpts   []encode.EncodeOpts
	encodeHook   func(ctx context.Context)
	reportDecode bool // call opts.ProgressFn from the mock
	concatCalls  int
	concatErr    error
	muxCalls     int
	muxErr       error
}

func (m *mockEnv) probe(ctx context.Context, path string) (*probe.VideoInfo, error) {
	return m.probeInfo, m.probeErr
}

func (m *mockEnv) decode(ctx context.Context, opts encode.DecodeOpts) (int, error) {
	m.decodeCalls++
	m.decodeOpts = append(m.decodeOpts, opts)
	if m.reportDecode && opts.ProgressFn != nil {
		opts.ProgressFn(15)
	}
	return m.decodeN, m.decodeErr
}

func (m *mockEnv) interp(ctx context.Context, opts interpolate.RifeOpts) error {
	m.interpCalls++
	if m.interpErrOn > 0 {
		if m.interpCalls == m.interpErrOn {
			return m.interpErr
		}
		return nil
	}
	return m.interpErr
}

func (m *mockEnv) encode(ctx context.Context, opts encode.EncodeOpts) error {
	m.encodeCalls++
	m.encodeOpts = append(m.encodeOpts, opts)
	if m.encodeHook != nil {
		m.encodeHook(ctx)
	}
	if m.encodeErrOn > 0 {
		if m.encodeCalls == m.encodeErrOn {
			return m.encodeErr
		}
		return nil
	}
	return m.encodeErr
}

func (m *mockEnv) concat(ctx context.Context, opts encode.ConcatOpts) error {
	m.concatCalls++
	return m.concatErr
}

func (m *mockEnv) mux(ctx context.Context, videoPath, audioPath string) error {
	m.muxCalls++
	return m.muxErr
}

func newTestOrchestrator(t *testing.T, env *mockEnv, skipExisting bool) (*Orchestrator, *config.Config, chan Event) {
	t.Helper()
	cfg := config.Default()
	cfg.OutputDir = t.TempDir()
	cfg.SkipExisting = skipExisting
	caps := &system.Capabilities{HasVideoToolbox: false}
	events := make(chan Event, 256)
	o := New(cfg, caps, events)
	o.probeFn = env.probe
	o.decodeFn = env.decode
	o.interpFn = env.interp
	o.encodeFn = env.encode
	o.concatFn = env.concat
	o.muxFn = env.mux
	return o, cfg, events
}

// drain collects events until the channel closes.
func drain(events chan Event) []Event {
	var got []Event
	for ev := range events {
		got = append(got, ev)
	}
	return got
}

func TestOrchestrator_HappyPath(t *testing.T) {
	env := &mockEnv{
		probeInfo:    &probe.VideoInfo{Width: 320, Height: 240, FPS: 30, Duration: 1e9, FrameCount: 30},
		decodeN:      30,
		reportDecode: true,
	}
	o, _, events := newTestOrchestrator(t, env, false)
	input := filepath.Join(t.TempDir(), "travel.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(events)
	<-done

	types := []EventType{EventJobAdded, EventJobStarted, EventStageChange, EventProgress, EventStageChange, EventStageChange, EventProgress, EventStageChange, EventJobCompleted, EventBatchDone}
	if len(got) != len(types) {
		t.Fatalf("got %d events %v, want %d", len(got), got, len(types))
	}
	for i, want := range types {
		if got[i].Type != want {
			t.Errorf("event[%d] = %v, want %v", i, got[i].Type, want)
		}
	}

	// probe summary went into the first StageChange message
	if got[2].Stage != StageProbing || got[2].Message != "320x240, 30fps, 00:01" {
		t.Errorf("probing event = %+v", got[2])
	}
	// progress emitted from the decode mock
	if got[3].Stage != StageDecoding || got[3].Progress != 50 {
		t.Errorf("decode progress event = %+v", got[3])
	}
	// completion message carries the output path and durations
	last := got[len(got)-2]
	if last.Type != EventJobCompleted || last.Message == "" {
		t.Errorf("completed event = %+v", last)
	}
	if env.decodeCalls != 1 || env.interpCalls != 1 || env.encodeCalls != 1 {
		t.Errorf("calls = decode %d, interp %d, encode %d", env.decodeCalls, env.interpCalls, env.encodeCalls)
	}

	jobs := o.Jobs()
	if len(jobs) != 1 || jobs[0].State != JobDone {
		t.Errorf("jobs = %+v, want one done job", jobs)
	}
	if jobs[0].OutputPath == "" {
		t.Error("OutputPath not set on job")
	}
}

func TestOrchestrator_SkipExisting(t *testing.T) {
	env := &mockEnv{probeInfo: &probe.VideoInfo{FPS: 30, Duration: 1e9}}
	o, cfg, events := newTestOrchestrator(t, env, true)
	input := filepath.Join(t.TempDir(), "travel.mp4")

	// pre-create the output so the job gets skipped
	output := outputPathFor(input, cfg.OutputDir, 60)
	if err := os.WriteFile(output, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(events)
	<-done

	if env.decodeCalls != 0 {
		t.Errorf("decode called %d times, want 0", env.decodeCalls)
	}
	found := false
	for _, ev := range got {
		if ev.Type == EventJobSkipped {
			found = true
		}
	}
	if !found {
		t.Errorf("no EventJobSkipped in %+v", got)
	}
	if jobs := o.Jobs(); len(jobs) != 1 || jobs[0].State != JobSkipped {
		t.Errorf("jobs = %+v, want one skipped job", jobs)
	}
}

func TestOrchestrator_Failure(t *testing.T) {
	env := &mockEnv{
		probeInfo: &probe.VideoInfo{FPS: 30, Duration: 1e9},
		decodeErr: errors.New("decode boom"),
	}
	o, _, events := newTestOrchestrator(t, env, false)
	input := filepath.Join(t.TempDir(), "travel.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	err := o.Run(context.Background())
	close(events)
	<-done

	if err == nil {
		t.Fatal("Run returned nil, want error")
	}
	found := false
	for _, ev := range got {
		if ev.Type == EventJobFailed && ev.Error != nil {
			found = true
		}
	}
	if !found {
		t.Errorf("no EventJobFailed in %+v", got)
	}
	if jobs := o.Jobs(); len(jobs) != 1 || jobs[0].State != JobFailed {
		t.Errorf("jobs = %+v, want one failed job", jobs)
	}
}

func TestOrchestrator_Chunked(t *testing.T) {
	env := &mockEnv{
		probeInfo: &probe.VideoInfo{FPS: 30, Duration: 200 * 1e9, FrameCount: 6000},
		decodeN:   3000,
	}
	o, cfg, events := newTestOrchestrator(t, env, false)
	cfg.ChunkSize = 3000
	input := filepath.Join(t.TempDir(), "long.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(events)
	<-done

	if env.decodeCalls != 2 || env.interpCalls != 2 || env.encodeCalls != 2 {
		t.Errorf("calls = decode %d, interp %d, encode %d, want 2 each",
			env.decodeCalls, env.interpCalls, env.encodeCalls)
	}
	if env.concatCalls != 1 {
		t.Errorf("concat calls = %d, want 1", env.concatCalls)
	}
	if len(env.encodeOpts) != 2 || env.encodeOpts[0].ChunkIndex != 0 || env.encodeOpts[1].ChunkIndex != 1 {
		t.Errorf("chunk indices = %+v, want 0,1", env.encodeOpts)
	}
	if env.encodeOpts[0].OutputPath != "" || env.encodeOpts[0].OutputDir == "" {
		t.Errorf("chunk encode should write to OutputDir, got %+v", env.encodeOpts[0])
	}
	if len(env.decodeOpts) != 2 ||
		env.decodeOpts[0].StartFrame != 0 || env.decodeOpts[0].EndFrame != 3000 ||
		env.decodeOpts[1].StartFrame != 3000 || env.decodeOpts[1].EndFrame != 6000 {
		t.Errorf("decode bounds = %+v, want [0,3000) and [3000,6000)", env.decodeOpts)
	}
	for _, ev := range got {
		if ev.Type == EventJobFailed {
			t.Errorf("unexpected failure: %v", ev.Error)
		}
	}
	jobs := o.Jobs()
	if len(jobs) != 1 || jobs[0].State != JobDone {
		t.Errorf("jobs = %+v, want one done job", jobs)
	}
}

func TestOrchestrator_NoChunk(t *testing.T) {
	env := &mockEnv{
		probeInfo: &probe.VideoInfo{FPS: 30, Duration: 10 * 1e9, FrameCount: 300},
		decodeN:   300,
	}
	o, cfg, events := newTestOrchestrator(t, env, false)
	cfg.ChunkSize = 3000 // default: 300 frames fits in one chunk
	input := filepath.Join(t.TempDir(), "short.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	go func() {
		drain(events)
		close(done)
	}()

	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(events)
	<-done

	if env.decodeCalls != 1 || env.interpCalls != 1 || env.encodeCalls != 1 {
		t.Errorf("calls = decode %d, interp %d, encode %d, want 1 each",
			env.decodeCalls, env.interpCalls, env.encodeCalls)
	}
	if env.concatCalls != 0 {
		t.Errorf("concat calls = %d, want 0", env.concatCalls)
	}
	if len(env.encodeOpts) != 1 || env.encodeOpts[0].ChunkIndex != encode.NoChunk || env.encodeOpts[0].OutputPath == "" {
		t.Errorf("single encode opts = %+v, want final output", env.encodeOpts)
	}
}

func TestOrchestrator_ChunkFailure(t *testing.T) {
	env := &mockEnv{
		probeInfo:   &probe.VideoInfo{FPS: 30, Duration: 200 * 1e9, FrameCount: 6000},
		decodeN:     3000,
		interpErrOn: 1,
		interpErr:   errors.New("rife boom"),
	}
	o, cfg, events := newTestOrchestrator(t, env, false)
	cfg.ChunkSize = 3000
	input := filepath.Join(t.TempDir(), "long.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	err := o.Run(context.Background())
	close(events)
	<-done

	if err == nil {
		t.Fatal("Run returned nil, want error")
	}
	if env.encodeCalls != 0 {
		t.Errorf("encode called %d times, want 0", env.encodeCalls)
	}
	if env.concatCalls != 0 {
		t.Errorf("concat called %d times, want 0", env.concatCalls)
	}
	found := false
	for _, ev := range got {
		if ev.Type == EventJobFailed && ev.Error != nil {
			found = true
		}
	}
	if !found {
		t.Errorf("no EventJobFailed in %+v", got)
	}
}

func TestOrchestrator_ChunkCanceled(t *testing.T) {
	env := &mockEnv{
		probeInfo: &probe.VideoInfo{FPS: 30, Duration: 200 * 1e9, FrameCount: 6000},
		decodeN:   3000,
	}
	o, cfg, events := newTestOrchestrator(t, env, false)
	cfg.ChunkSize = 3000
	ctx, cancel := context.WithCancel(context.Background())
	env.encodeHook = func(c context.Context) { cancel() }
	input := filepath.Join(t.TempDir(), "long.mp4")

	o.Enqueue([]string{input})
	done := make(chan struct{})
	var got []Event
	go func() {
		got = drain(events)
		close(done)
	}()

	err := o.Run(ctx)
	close(events)
	<-done

	if err == nil {
		t.Fatal("Run returned nil, want cancellation error")
	}
	// chunk 0 encoded, chunk 1 must not start
	if env.encodeCalls != 1 || env.interpCalls != 1 {
		t.Errorf("calls = interp %d, encode %d, want chunk 0 only", env.interpCalls, env.encodeCalls)
	}
	if env.concatCalls != 0 {
		t.Errorf("concat called %d times, want 0", env.concatCalls)
	}
	found := false
	for _, ev := range got {
		if ev.Type == EventJobFailed && ev.Error != nil {
			found = true
		}
	}
	if !found {
		t.Errorf("no EventJobFailed in %+v", got)
	}
}

func TestOrchestrator_JobsSnapshot(t *testing.T) {
	env := &mockEnv{probeInfo: &probe.VideoInfo{FPS: 30, Duration: 1e9}}
	o, _, _ := newTestOrchestrator(t, env, false)
	o.Enqueue([]string{"/a.mp4", "/b.mp4"})

	jobs := o.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].InputPath != "/a.mp4" || jobs[1].InputPath != "/b.mp4" {
		t.Errorf("order wrong: %v %v", jobs[0].InputPath, jobs[1].InputPath)
	}
	if jobs[0].State != JobPending {
		t.Errorf("state = %v, want pending", jobs[0].State)
	}
	// snapshot must be detached from the live job
	jobs[0].State = JobDone
	if live := o.Jobs()[0]; live.State != JobPending {
		t.Errorf("mutating snapshot leaked: %v", live.State)
	}
}
