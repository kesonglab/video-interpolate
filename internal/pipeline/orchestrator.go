package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/encode"
	"github.com/kesonglab/video-interpolate/internal/interpolate"
	"github.com/kesonglab/video-interpolate/internal/probe"
	"github.com/kesonglab/video-interpolate/internal/system"
)

type Orchestrator struct {
	cfg      *config.Config
	caps     *system.Capabilities
	events   chan<- Event
	mu       sync.Mutex
	jobs     map[string]*Job
	order    []string
	canceled atomic.Bool

	// injectable for tests
	probeFn  func(ctx context.Context, path string) (*probe.VideoInfo, error)
	decodeFn func(ctx context.Context, opts encode.DecodeOpts) (int, error)
	interpFn func(ctx context.Context, opts interpolate.RifeOpts) error
	encodeFn func(ctx context.Context, opts encode.EncodeOpts) error
	concatFn func(ctx context.Context, opts encode.ConcatOpts) error
	muxFn    func(ctx context.Context, videoPath, audioPath string) error
}

type Event struct {
	JobID        string
	Type         EventType
	Stage        Stage
	Progress     float64
	FPS          float64
	ETA          time.Duration
	Message      string
	Error        error
	Time         time.Time
	ChunkCurrent int // 0 = no chunking
	ChunkTotal   int // 1 = no chunking
}

type EventType int

const (
	EventJobAdded EventType = iota
	EventJobStarted
	EventStageChange
	EventProgress
	EventJobCompleted
	EventJobFailed
	EventJobSkipped
	EventBatchDone
)

type Stage string

const (
	StageProbing       Stage = "probing"
	StageDecoding      Stage = "decoding"
	StageInterpolating Stage = "interpolating"
	StageEncoding      Stage = "encoding"
)

var jobSeq atomic.Int64

func newJobID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), jobSeq.Add(1))
}

func New(cfg *config.Config, caps *system.Capabilities, events chan<- Event) *Orchestrator {
	o := &Orchestrator{
		cfg:    cfg,
		caps:   caps,
		events: events,
		jobs:   map[string]*Job{},
	}
	o.probeFn = probe.Probe
	o.decodeFn = encode.DecodeToFrames
	o.interpFn = interpolate.Interpolate
	o.encodeFn = encode.EncodeFromFrames
	o.concatFn = encode.ConcatChunks
	o.muxFn = encode.MuxAudio
	return o
}

func (o *Orchestrator) emit(ev Event) {
	if o.events == nil {
		return
	}
	ev.Time = time.Now()
	select {
	case o.events <- ev:
	default:
		// channel full, drop
	}
}

// Enqueue assigns IDs and stages the files as pending jobs.
func (o *Orchestrator) Enqueue(paths []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, p := range paths {
		id := newJobID()
		o.jobs[id] = &Job{ID: id, InputPath: p, State: JobPending}
		o.order = append(o.order, id)
		o.emit(Event{JobID: id, Type: EventJobAdded})
	}
}

// Run processes all jobs serially. Jobs run one at a time in Phase 1b.
// Returns the first error encountered; processing continues on failure.
func (o *Orchestrator) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	err := o.runPending(ctx)
	o.emit(Event{Type: EventBatchDone})
	return err
}

// RunLoop keeps draining the job queue until ctx is cancelled. wakeup is
// signaled whenever new jobs are enqueued so RunLoop doesn't busy-spin.
// Used by watch mode, where jobs arrive over time.
func (o *Orchestrator) RunLoop(ctx context.Context, wakeup <-chan struct{}) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var firstErr error
	for {
		if err := o.runPending(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		select {
		case <-ctx.Done():
			o.emit(Event{Type: EventBatchDone})
			return firstErr
		case <-wakeup:
		}
	}
}

// runPending processes whatever jobs are queued, skipping ones already done.
func (o *Orchestrator) runPending(ctx context.Context) error {
	var firstErr error
	for _, id := range o.order {
		if ctx.Err() != nil || o.canceled.Load() {
			break
		}
		job := o.jobs[id]
		if job == nil || job.Terminal() {
			continue
		}
		if err := o.processJob(ctx, job); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// DryRun probes each job and returns the commands it would run, without executing them.
func (o *Orchestrator) DryRun(ctx context.Context) ([]string, error) {
	var lines []string
	for i, id := range o.order {
		if ctx.Err() != nil || o.canceled.Load() {
			break
		}
		job := o.jobs[id]
		lines = append(lines, fmt.Sprintf("[%d/%d] %s", i+1, len(o.order), filepath.Base(job.InputPath)))
		plan, skip, err := o.planJob(ctx, job)
		if err != nil {
			return lines, fmt.Errorf("%s: %w", job.InputPath, err)
		}
		if skip {
			lines = append(lines, "  [skip] output exists: "+plan.output)
			continue
		}
		lines = append(lines, "  [probe]  ffprobe -v error -print_format json -show_format -show_streams "+plan.info.Path)
		if plan.hasAudio && plan.audioMode != "drop" {
			lines = append(lines, "  [audio]  ffmpeg -y -v error -i "+plan.info.Path+" -vn -acodec copy "+plan.audioPath)
		}
		chunkDir := ""
		if plan.totalChunks > 1 {
			chunkDir = filepath.Join(plan.tmp, "chunks")
		}
		for ci := 0; ci < plan.totalChunks; ci++ {
			start, end := plan.bounds(ci)
			if plan.totalChunks > 1 {
				lines = append(lines, fmt.Sprintf("  [chunk %d/%d] frames %d-%d", ci+1, plan.totalChunks, start, end))
			}
			lines = append(lines, "    [decode] ffmpeg "+strings.Join(encode.DecodeCommand(plan.decodeOpts(ci)), " "))
			lines = append(lines, "    [rife]   "+strings.Join(interpolate.RifeCommand(plan.rifeOpts(ci)), " "))
			lines = append(lines, "    [encode] ffmpeg "+strings.Join(encode.EncodeCommand(plan.encodeOpts(ci, chunkDir)), " "))
		}
		if plan.totalChunks > 1 {
			lines = append(lines, "  [concat] ffmpeg -y -v error -f concat -safe 0 -i "+filepath.Join(chunkDir, "list.txt")+" -c copy "+plan.output)
		}
		lines = append(lines, "  [cleanup] rm -rf "+plan.tmp)
	}
	return lines, nil
}

// Jobs returns a snapshot of all jobs in submission order.
func (o *Orchestrator) Jobs() []*Job {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]*Job, 0, len(o.order))
	for _, id := range o.order {
		if j, ok := o.jobs[id]; ok {
			s := j.Snapshot()
			out = append(out, &s)
		}
	}
	return out
}

// Cancel stops the current job from starting the next one.
func (o *Orchestrator) Cancel() {
	o.canceled.Store(true)
}

// jobPlan holds everything a job needs to run, resolved once.
type jobPlan struct {
	job         *Job
	info        *probe.VideoInfo
	targetFPS   float64
	output      string
	tmp         string
	hasAudio    bool
	audioPath   string
	audioMode   string
	encoder     string
	quality     int
	hwDecoder   string
	format      string
	jpgQuality  int
	gpu         int
	threads     string
	rifeModel   string
	rifeBin     string
	multiplier  int
	totalFrames int
	chunkSize   int
	totalChunks int
}

func (o *Orchestrator) planJob(ctx context.Context, job *Job) (*jobPlan, bool, error) {
	info, err := o.probeFn(ctx, job.InputPath)
	if err != nil {
		return nil, false, fmt.Errorf("probe %s: %w", job.InputPath, err)
	}
	targetFPS := info.FPS * float64(o.cfg.Multiplier)
	if o.cfg.TargetFPS > 0 {
		targetFPS = o.cfg.TargetFPS
	}
	if targetFPS <= 0 {
		targetFPS = info.FPS
	}

	output := outputPathFor(job.InputPath, o.cfg.OutputDir, targetFPS)
	if fileExists(output) {
		if o.cfg.SkipExisting {
			return nil, true, nil
		}
		output = uniqueOutputPath(output)
	}

	tmp := filepath.Join(os.TempDir(), "vif-"+job.ID)
	audio := filepath.Join(tmp, "audio.m4a")

	audioMode := o.cfg.Audio
	audioPath := audio
	if !info.HasAudio {
		audioMode = "drop"
		audioPath = ""
	}

	totalFrames := estimateTotalFrames(info)
	chunkSize := o.cfg.ChunkSize
	totalChunks := 1
	if chunkSize > 0 && totalFrames > chunkSize {
		totalChunks = (totalFrames + chunkSize - 1) / chunkSize
	}

	return &jobPlan{
		job:         job,
		info:        info,
		targetFPS:   targetFPS,
		output:      output,
		tmp:         tmp,
		hasAudio:    info.HasAudio,
		audioPath:   audioPath,
		audioMode:   audioMode,
		encoder:     resolveEncoder(o.cfg.Encoder, o.caps),
		quality:     encode.QualityFor(resolveEncoder(o.cfg.Encoder, o.caps), o.cfg.Quality),
		hwDecoder:   resolveHW(o.cfg.HWDecode, o.caps),
		format:      o.cfg.Format,
		jpgQuality:  o.cfg.JPGQuality,
		gpu:         o.cfg.GPU,
		threads:     o.cfg.Threads,
		rifeModel:   o.cfg.RifeModel,
		rifeBin:     o.cfg.RifeBin,
		multiplier:  o.cfg.Multiplier,
		totalFrames: totalFrames,
		chunkSize:   chunkSize,
		totalChunks: totalChunks,
	}, false, nil
}

// bounds returns the half-open frame range [start, end) for chunk i.
func (p *jobPlan) bounds(i int) (start, end int) {
	if p.totalChunks <= 1 {
		return 0, p.totalFrames
	}
	start = i * p.chunkSize
	end = start + p.chunkSize
	if end > p.totalFrames {
		end = p.totalFrames
	}
	return start, end
}

// workDir is the per-chunk temp dir; the shared tmp for a single chunk.
func (p *jobPlan) workDir(i int) string {
	if p.totalChunks <= 1 {
		return p.tmp
	}
	return filepath.Join(p.tmp, fmt.Sprintf("chunk_%03d", i))
}

func (p *jobPlan) decodeOpts(i int) encode.DecodeOpts {
	start, end := p.bounds(i)
	return encode.DecodeOpts{
		Input:      p.info.Path,
		OutputDir:  filepath.Join(p.workDir(i), "in"),
		HWDecoder:  p.hwDecoder,
		Format:     p.format,
		JPGQuality: p.jpgQuality,
		StartFrame: start,
		EndFrame:   end,
		FPS:        p.info.FPS,
	}
}

func (p *jobPlan) rifeOpts(i int) interpolate.RifeOpts {
	return interpolate.RifeOpts{
		InputDir:   filepath.Join(p.workDir(i), "in"),
		OutputDir:  filepath.Join(p.workDir(i), "out"),
		GPU:        p.gpu,
		Threads:    p.threads,
		ModelPath:  p.rifeModel,
		Format:     p.format,
		Multiplier: p.multiplier,
		Bin:        p.rifeBin,
	}
}

func (p *jobPlan) encodeOpts(i int, chunkDir string) encode.EncodeOpts {
	opts := encode.EncodeOpts{
		InputDir:   filepath.Join(p.workDir(i), "out"),
		InputFPS:   p.targetFPS,
		Encoder:    p.encoder,
		Quality:    p.quality,
		PixelFmt:   "yuv420p",
		Format:     p.format,
		AudioMode:  p.audioMode,
		ChunkIndex: encode.NoChunk,
	}
	if chunkDir != "" {
		opts.OutputDir = chunkDir
		opts.ChunkIndex = i
	} else {
		opts.OutputPath = p.output
		if p.hasAudio && p.audioMode != "drop" {
			opts.AudioPath = p.audioPath
		}
	}
	return opts
}

func (o *Orchestrator) processJob(ctx context.Context, job *Job) error {
	job.StartTime = time.Now()
	o.emit(Event{JobID: job.ID, Type: EventJobStarted})
	job.SetState(JobProbing)

	plan, skip, err := o.planJob(ctx, job)
	if err != nil {
		o.fail(job, err)
		return err
	}
	if skip {
		job.SetState(JobSkipped)
		job.EndTime = time.Now()
		o.emit(Event{JobID: job.ID, Type: EventJobSkipped, Message: "output exists, skipped"})
		return nil
	}
	job.OutputPath = plan.output
	job.SetStage(StageProbing)
	o.emit(Event{JobID: job.ID, Type: EventStageChange, Stage: StageProbing, Message: probeSummary(plan.info)})

	if err := os.MkdirAll(filepath.Dir(plan.output), 0o755); err != nil {
		o.fail(job, fmt.Errorf("create output dir: %w", err))
		return err
	}
	if err := os.MkdirAll(plan.tmp, 0o755); err != nil {
		o.fail(job, fmt.Errorf("create temp dir: %w", err))
		return err
	}
	defer os.RemoveAll(plan.tmp)

	// audio extraction runs once, chunked or not
	if plan.hasAudio && plan.audioMode != "drop" {
		if ok, err := encode.ExtractAudio(ctx, job.InputPath, plan.audioPath); err != nil || !ok {
			plan.audioPath = ""
			plan.audioMode = "drop"
		}
	}

	if plan.totalChunks > 1 {
		if err := o.runChunked(ctx, plan); err != nil {
			o.fail(job, err)
			return err
		}
	} else {
		if err := o.runChunk(ctx, plan, 0, ""); err != nil {
			o.fail(job, err)
			return err
		}
	}

	job.SetState(JobDone)
	job.EndTime = time.Now()
	outDur := plan.info.Duration
	if plan.info.Duration > 0 && plan.info.FPS > 0 {
		outDur = time.Duration(float64(plan.info.Duration) * plan.targetFPS / plan.info.FPS)
	}
	msg := fmt.Sprintf("%s (%s → %s)", plan.output, fmtDuration(plan.info.Duration), fmtDuration(outDur))
	o.emit(Event{JobID: job.ID, Type: EventJobCompleted, Message: msg})
	return nil
}

// runChunked processes each chunk independently, then concats and muxes audio.
func (o *Orchestrator) runChunked(ctx context.Context, plan *jobPlan) error {
	chunkDir := filepath.Join(plan.tmp, "chunks")
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}
	defer os.RemoveAll(chunkDir)

	for i := 0; i < plan.totalChunks; i++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("canceled: %w", err)
		}
		o.emit(Event{JobID: plan.job.ID, Type: EventStageChange, Stage: StageDecoding,
			Message:      fmt.Sprintf("chunk %d/%d", i+1, plan.totalChunks),
			ChunkCurrent: i + 1, ChunkTotal: plan.totalChunks})
		if err := o.runChunk(ctx, plan, i, chunkDir); err != nil {
			return err
		}
		o.emit(Event{JobID: plan.job.ID, Type: EventProgress, Stage: StageInterpolating,
			Progress:     float64(i+1) / float64(plan.totalChunks) * 100,
			ChunkCurrent: i + 1, ChunkTotal: plan.totalChunks})
	}

	o.emit(Event{JobID: plan.job.ID, Type: EventStageChange, Stage: StageEncoding,
		Message:      "concatenating chunks",
		ChunkCurrent: plan.totalChunks, ChunkTotal: plan.totalChunks})
	if err := o.concatFn(ctx, encode.ConcatOpts{
		ChunkDir:   chunkDir,
		OutputPath: plan.output,
		ChunkCount: plan.totalChunks,
	}); err != nil {
		os.Remove(plan.output)
		return err
	}

	if plan.hasAudio && plan.audioPath != "" && plan.audioMode != "drop" {
		if err := o.muxFn(ctx, plan.output, plan.audioPath); err != nil {
			os.Remove(plan.output)
			return fmt.Errorf("mux audio: %w", err)
		}
	}
	return nil
}

// runChunk decodes, interpolates and encodes one chunk (or the whole video
// when chunkDir is empty). Each chunk's frame dirs are removed on exit.
func (o *Orchestrator) runChunk(ctx context.Context, plan *jobPlan, i int, chunkDir string) error {
	if plan.totalChunks > 1 {
		work := plan.workDir(i)
		if err := os.MkdirAll(work, 0o755); err != nil {
			return fmt.Errorf("create chunk temp: %w", err)
		}
		defer os.RemoveAll(work)
	}
	chunkCurrent, chunkTotal := i+1, plan.totalChunks

	// decode
	plan.job.SetState(JobDecoding)
	stageStart := time.Now()
	start, end := plan.bounds(i)
	decodeTotal := end - start
	if decodeTotal <= 0 {
		decodeTotal = plan.totalFrames
	}
	dec := plan.decodeOpts(i)
	dec.ProgressFn = func(done int) {
		o.progress(plan.job, StageDecoding, stageStart, done, decodeTotal, chunkCurrent, chunkTotal)
	}
	decoded, err := o.decodeFn(ctx, dec)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	plan.job.SetStage(StageDecoding)
	o.emit(Event{JobID: plan.job.ID, Type: EventStageChange, Stage: StageDecoding,
		Message:      fmt.Sprintf("%d frames", decoded),
		ChunkCurrent: chunkCurrent, ChunkTotal: chunkTotal})

	// rife
	plan.job.SetState(JobInterpolating)
	stageStart = time.Now()
	expected := interpolate.MultForFrames(decoded, plan.multiplier)
	rife := plan.rifeOpts(i)
	rife.ProgressFn = func(done, total int) {
		o.progress(plan.job, StageInterpolating, stageStart, done, expected, chunkCurrent, chunkTotal)
	}
	if err := o.interpFn(ctx, rife); err != nil {
		return fmt.Errorf("rife: %w", err)
	}
	plan.job.SetStage(StageInterpolating)
	o.emit(Event{JobID: plan.job.ID, Type: EventStageChange, Stage: StageInterpolating, ChunkCurrent: chunkCurrent, ChunkTotal: chunkTotal})
	o.emit(Event{JobID: plan.job.ID, Type: EventProgress, Stage: StageInterpolating, Progress: 100, ChunkCurrent: chunkCurrent, ChunkTotal: chunkTotal})

	// encode
	plan.job.SetState(JobEncoding)
	stageStart = time.Now()
	enc := plan.encodeOpts(i, chunkDir)
	enc.ProgressFn = func(frame int) {
		o.progress(plan.job, StageEncoding, stageStart, frame, expected, chunkCurrent, chunkTotal)
	}
	if err := o.encodeFn(ctx, enc); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	plan.job.SetStage(StageEncoding)
	o.emit(Event{JobID: plan.job.ID, Type: EventStageChange, Stage: StageEncoding, ChunkCurrent: chunkCurrent, ChunkTotal: chunkTotal})
	return nil
}

func (o *Orchestrator) progress(job *Job, stage Stage, start time.Time, done, total, chunkCurrent, chunkTotal int) {
	if total <= 0 {
		return
	}
	pct := float64(done) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}
	elapsed := time.Since(start).Seconds()
	var eta time.Duration
	var fps float64
	if elapsed > 0 && done > 0 {
		rate := float64(done) / elapsed
		fps = rate
		if done < total {
			eta = time.Duration(float64(total-done)/rate) * time.Second
		}
	}
	o.emit(Event{JobID: job.ID, Type: EventProgress, Stage: stage, Progress: pct, FPS: fps, ETA: eta, ChunkCurrent: chunkCurrent, ChunkTotal: chunkTotal})
}

func (o *Orchestrator) fail(job *Job, err error) {
	job.SetState(JobFailed)
	job.SetError(err)
	job.EndTime = time.Now()
	o.emit(Event{JobID: job.ID, Type: EventJobFailed, Error: err})
}

func outputPathFor(input, outputDir string, targetFPS float64) string {
	outputDir = expandHome(outputDir)
	base := filepath.Base(input)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(outputDir, fmt.Sprintf("%s_%.0ffps.mp4", stem, targetFPS))
}

func uniqueOutputPath(p string) string {
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for n := 1; ; n++ {
		cand := fmt.Sprintf("%s_%d%s", stem, n, ext)
		if !fileExists(cand) {
			return cand
		}
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

func resolveEncoder(enc string, caps *system.Capabilities) string {
	if enc == "" || enc == "auto" {
		if caps != nil && caps.BestEncoder != "" {
			return caps.BestEncoder
		}
		return "libx264"
	}
	return enc
}

func resolveHW(mode string, caps *system.Capabilities) string {
	switch mode {
	case "", "off":
		return ""
	case "auto":
		if caps != nil && caps.HasVideoToolbox {
			return "videotoolbox"
		}
		return ""
	default:
		return mode
	}
}

func estimateTotalFrames(info *probe.VideoInfo) int {
	if info.FrameCount > 0 {
		return info.FrameCount
	}
	if info.FPS > 0 && info.Duration > 0 {
		return int(info.Duration.Seconds() * info.FPS)
	}
	return 0
}

func probeSummary(info *probe.VideoInfo) string {
	return fmt.Sprintf("%dx%d, %.0ffps, %s", info.Width, info.Height, info.FPS, fmtDuration(info.Duration))
}

func fmtDuration(d time.Duration) string {
	if d <= 0 {
		return "00:00"
	}
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
