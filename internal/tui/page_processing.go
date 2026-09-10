package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

const (
	// splitWidth is where the dashboard switches from stacked to two columns.
	splitWidth = 80
	// fpsHistory is how many progress samples the sparkline keeps.
	fpsHistory = 30
)

// jobState is the TUI-side view of one pipeline job.
type jobState struct {
	ID         string
	InputPath  string
	OutputPath string
	Status     string // pending / active / done / failed / skipped
	Error      string
	StartTime  time.Time
	EndTime    time.Time
}

type processingState struct {
	pageID
	ctx *SharedContext

	currentJobID string
	jobs         map[string]*jobState
	jobOrder     []string

	rifeFPSHist []float64

	cpuPct float64
	memPct float64
	gpuPct float64

	popup      Popup
	showMascot bool

	rifeProgress float64
	rifeStage    string
	rifeETA      time.Duration
	rifeSpeed    float64

	encodeProgress float64
	encodeStage    string
	encodeETA      time.Duration
	encodeSize     int64
	encodeBitrate  float64

	chunkCurrent int
	chunkTotal   int

	sourceWidth  int
	sourceHeight int
	sourceFPS    float64
	sourceDur    time.Duration
	sourceFrames int
}

func NewProcessingPage(ctx *SharedContext) Page {
	return &processingState{
		ctx:       ctx,
		jobs:      map[string]*jobState{},
		rifeStage: "waiting",
	}
}

func (p *processingState) Init() tea.Cmd {
	if p.ctx.SourceFPS == 0 && len(p.ctx.FileInfo) > 0 {
		if _, _, fps, _ := parseMediaSummary(p.ctx.FileInfo[0]); fps > 0 {
			p.ctx.SourceFPS = fps
			p.sourceFPS = fps
		}
	}
	return tea.Batch(waitForEvent(p.ctx.EventsCh), systemStatsTick())
}

// waitForEvent reads one pipeline event; a closed channel means the run is over.
func waitForEvent(ch <-chan pipeline.Event) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return batchDoneMsg{}
		}
		return pipelineEventMsg(ev)
	}
}

func (p *processingState) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case pipelineEventMsg:
		return p, p.handleEvent(pipeline.Event(msg))
	case popupResultMsg:
		return p, p.handlePopupResult(msg)
	case systemStatsTickMsg:
		return p, tea.Batch(readSystemStatsCmd(), systemStatsTick())
	case systemStatsMsg:
		p.cpuPct, p.memPct, p.gpuPct = msg.CPU, msg.Mem, msg.GPU
		return p, nil
	case tea.KeyMsg:
		return p, p.handleKey(msg)
	}
	return p, nil
}

func (p *processingState) handleKey(msg tea.KeyMsg) tea.Cmd {
	if p.popup.Active() {
		var cmd tea.Cmd
		p.popup, cmd = p.popup.Update(msg)
		return cmd
	}
	key := msg.Key()
	switch key.Text {
	case "q":
		p.popup = NewPopup("quit", "Quit", "Quit after the current file?", []string{"Cancel", "Quit"}, render.Warn).Open()
	case "c":
		p.popup = NewPopup("cancel", "Cancel job", "Cancel the current job?", []string{"No", "Yes"}, render.Danger).Open()
	case "n":
		p.popup = NewPopup("skip", "Skip file", "Skip the current file?", []string{"No", "Yes"}, render.Warn).Open()
	case "m":
		p.showMascot = !p.showMascot
	case "p":
		// TODO: pause/resume needs orchestrator support
	}
	if key.Code == tea.KeyEsc {
		p.popup = NewPopup("quit", "Quit", "Quit after the current file?", []string{"Cancel", "Quit"}, render.Warn).Open()
	}
	return nil
}

func (p *processingState) handlePopupResult(msg popupResultMsg) tea.Cmd {
	if !msg.Chosen {
		return nil
	}
	switch msg.ID {
	case "quit":
		if msg.ButtonIndex == 1 {
			p.cancelRun()
			return tea.Quit
		}
	case "cancel":
		if msg.ButtonIndex == 1 {
			p.cancelRun()
		}
	case "skip":
		// TODO: orchestrator has no per-job skip yet
	}
	return nil
}

// cancelRun aborts the current job and any queued ones.
func (p *processingState) cancelRun() {
	if p.ctx.CancelRun != nil {
		p.ctx.CancelRun()
	}
	if p.ctx.Orch != nil {
		p.ctx.Orch.Cancel()
	}
}

func (p *processingState) handleEvent(ev pipeline.Event) tea.Cmd {
	switch ev.Type {
	case pipeline.EventJobAdded:
		p.addJob(ev.JobID)
	case pipeline.EventJobStarted:
		if j := p.jobs[ev.JobID]; j != nil {
			j.Status = "active"
			j.StartTime = ev.Time
		}
		p.currentJobID = ev.JobID
		p.rifeProgress, p.encodeProgress = 0, 0
		p.rifeETA, p.encodeETA = 0, 0
		p.rifeFPSHist = nil
		p.chunkCurrent, p.chunkTotal = 0, 0
	case pipeline.EventStageChange:
		p.applyStage(ev)
	case pipeline.EventProgress:
		p.applyProgress(ev)
	case pipeline.EventJobCompleted:
		p.completeJob(ev)
	case pipeline.EventJobFailed:
		if j := p.jobs[ev.JobID]; j != nil {
			j.Status = "failed"
			if ev.Error != nil {
				j.Error = ev.Error.Error()
			}
			j.EndTime = ev.Time
		}
	case pipeline.EventJobSkipped:
		if j := p.jobs[ev.JobID]; j != nil {
			j.Status = "skipped"
			j.Error = ev.Message
			j.EndTime = ev.Time
		}
	case pipeline.EventBatchDone:
		return func() tea.Msg { return batchDoneMsg{Summary: p.summary()} }
	}
	return waitForEvent(p.ctx.EventsCh)
}

func (p *processingState) addJob(id string) {
	if _, ok := p.jobs[id]; ok {
		return
	}
	input := ""
	if i := len(p.jobOrder); i < len(p.ctx.Files) {
		input = p.ctx.Files[i]
	}
	target := 0.0
	if p.ctx.SourceFPS > 0 {
		target = p.ctx.SourceFPS * float64(p.ctx.Multiplier)
	}
	p.jobs[id] = &jobState{
		ID:         id,
		InputPath:  input,
		OutputPath: expectedOutputPath(input, p.ctx.OutputDir, target),
		Status:     "pending",
	}
	p.jobOrder = append(p.jobOrder, id)
}

func (p *processingState) applyStage(ev pipeline.Event) {
	p.trackChunk(ev)
	switch ev.Stage {
	case pipeline.StageProbing:
		w, h, fps, dur := parseMediaSummary(ev.Message)
		if w > 0 {
			p.sourceWidth, p.sourceHeight = w, h
		}
		if fps > 0 {
			p.sourceFPS = fps
			p.ctx.SourceFPS = fps
		}
		if dur > 0 {
			p.sourceDur = dur
		}
		if fps > 0 && dur > 0 {
			p.sourceFrames = int(dur.Seconds() * fps)
		}
		p.rifeStage = "probing"
	case pipeline.StageDecoding:
		p.rifeStage = "decoding frames" + p.chunkLabel()
	case pipeline.StageInterpolating:
		p.rifeStage = "generating frames" + p.chunkLabel()
	case pipeline.StageEncoding:
		p.encodeStage = "writing video" + p.chunkLabel()
	}
}

// trackChunk remembers the current chunk so stage labels can show "chunk 3/10".
func (p *processingState) trackChunk(ev pipeline.Event) {
	if ev.ChunkTotal > 1 {
		p.chunkCurrent, p.chunkTotal = ev.ChunkCurrent, ev.ChunkTotal
	}
}

func (p *processingState) chunkLabel() string {
	if p.chunkTotal > 1 {
		return fmt.Sprintf(" (chunk %d/%d)", p.chunkCurrent, p.chunkTotal)
	}
	return ""
}

func (p *processingState) applyProgress(ev pipeline.Event) {
	p.trackChunk(ev)
	switch ev.Stage {
	case pipeline.StageDecoding:
		p.rifeStage = "decoding frames" + p.chunkLabel()
		p.rifeProgress = ev.Progress
		if ev.FPS > 0 {
			p.rifeSpeed = ev.FPS
		}
		p.rifeETA = ev.ETA
	case pipeline.StageInterpolating:
		p.rifeStage = "generating frames" + p.chunkLabel()
		p.rifeProgress = ev.Progress
		if ev.FPS > 0 {
			p.rifeSpeed = ev.FPS
		}
		p.rifeETA = ev.ETA
		p.pushFPS(ev.FPS)
	case pipeline.StageEncoding:
		p.encodeStage = "writing video" + p.chunkLabel()
		p.encodeProgress = ev.Progress
		p.encodeETA = ev.ETA
		p.encodeBitrate = nominalBitrateMbps(p.ctx.Quality)
		outDur := p.sourceDur * time.Duration(p.ctx.Multiplier)
		total := int64(p.encodeBitrate / 8 * 1e6 * outDur.Seconds())
		p.encodeSize = int64(float64(total) * ev.Progress / 100)
	}
}

func (p *processingState) pushFPS(fps float64) {
	if fps <= 0 {
		return
	}
	p.rifeFPSHist = append(p.rifeFPSHist, fps)
	if len(p.rifeFPSHist) > fpsHistory {
		p.rifeFPSHist = p.rifeFPSHist[len(p.rifeFPSHist)-fpsHistory:]
	}
}

func (p *processingState) completeJob(ev pipeline.Event) {
	j := p.jobs[ev.JobID]
	if j == nil {
		return
	}
	j.Status = "done"
	j.EndTime = ev.Time
	if path := outputPathFromMessage(ev.Message); path != "" {
		j.OutputPath = path
	}
	if j.OutputPath != "" {
		if fi, err := os.Stat(j.OutputPath); err == nil {
			p.encodeSize = fi.Size()
		}
	}
}

func (p *processingState) summary() BatchSummary {
	s := BatchSummary{}
	var start, end time.Time
	for _, id := range p.jobOrder {
		j := p.jobs[id]
		if j == nil {
			continue
		}
		out := JobOutput{Path: j.OutputPath, Status: j.Status, Error: j.Error}
		switch j.Status {
		case "done":
			s.Success++
			if j.OutputPath != "" {
				if fi, err := os.Stat(j.OutputPath); err == nil {
					out.Size = fi.Size()
				}
			}
		case "failed":
			s.Failed++
		default:
			s.Skipped++
		}
		out.Duration = j.EndTime.Sub(j.StartTime)
		if !j.StartTime.IsZero() && (start.IsZero() || j.StartTime.Before(start)) {
			start = j.StartTime
		}
		if !j.EndTime.IsZero() && j.EndTime.After(end) {
			end = j.EndTime
		}
		s.Outputs = append(s.Outputs, out)
	}
	if !start.IsZero() && !end.IsZero() {
		s.Elapsed = end.Sub(start)
	}
	return s
}

func (p *processingState) View() tea.View {
	width := p.ctx.Width
	if width < 40 {
		width = 80
	}
	height := p.ctx.Height
	if height < 10 {
		height = 24
	}

	cw := (width - 4) / 2
	if cw < 24 {
		cw = 24
	}

	sourceCard := components.Card(render.IconSource, "Source", p.sourceLines(), cw)
	outputCard := components.Card(render.IconOutput, "Output", p.outputLines(), cw)
	rifeCard := components.Card(render.IconRife, "RIFE", p.rifeLines(), cw)
	encodeCard := components.Card(render.IconEncode, "Encode", p.encodeLines(), cw)
	sysCard := components.Card(render.IconSystem, "System", p.systemLines(), cw)

	left := lipgloss.JoinVertical(lipgloss.Left, sourceCard, "", outputCard)
	right := lipgloss.JoinVertical(lipgloss.Left, rifeCard, "", encodeCard, "", sysCard)

	var cols string
	if width >= splitWidth {
		cols = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	} else {
		cols = lipgloss.JoinVertical(lipgloss.Left, left, "", right)
	}

	parts := []string{
		p.header(width),
		components.Separator(width),
		"",
		cols,
		"",
	}
	if p.showMascot {
		parts = append(parts, mascot(p.ctx.MascotFrame, render.Primary), "")
	}
	parts = append(parts,
		components.Separator(width),
		components.KeyHint([]components.HintPair{
			{Key: "c", Desc: "cancel current"},
			{Key: "n", Desc: "skip"},
			{Key: "p", Desc: "pause"},
			{Key: "m", Desc: "toggle mascot"},
			{Key: "q", Desc: "quit"},
		}),
	)

	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if p.popup.Active() {
		return tea.NewView(p.popup.View(body, width, height))
	}
	return tea.NewView(body)
}

func (p *processingState) header(width int) string {
	done := 0
	for _, id := range p.jobOrder {
		if j := p.jobs[id]; j != nil && j.Status != "pending" && j.Status != "active" {
			done++
		}
	}
	style := render.Warn
	if len(p.jobOrder) > 0 && done == len(p.jobOrder) {
		style = render.OK
	}
	status := fmt.Sprintf("%d/%d", done, len(p.jobOrder))
	return components.Header("Interpolate", "●", "Batch "+status, style, p.hardwareText(), width)
}

func (p *processingState) sourceLines() []string {
	file := "—"
	if j := p.currentJob(); j != nil {
		file = filepath.Base(j.InputPath)
	}
	res := "—"
	if p.sourceWidth > 0 {
		res = fmt.Sprintf("%d × %d", p.sourceWidth, p.sourceHeight)
	}
	fps := "—"
	if p.sourceFPS > 0 {
		fps = fmt.Sprintf("%.2f", p.sourceFPS)
	}
	return []string{
		row("File", file),
		row("Res", res),
		row("FPS", fps),
		row("Dur", formatDuration(p.sourceDur)),
		row("Frames", fmt.Sprintf("%d", p.sourceFrames)),
	}
}

func (p *processingState) outputLines() []string {
	target := "—"
	if p.ctx.SourceFPS > 0 {
		target = fmt.Sprintf("%.0f fps", p.ctx.SourceFPS*float64(p.ctx.Multiplier))
	}
	out := "—"
	if j := p.currentJob(); j != nil && j.OutputPath != "" {
		out = j.OutputPath
	}
	return []string{
		row("Mult", fmt.Sprintf("x%d", p.ctx.Multiplier)),
		row("Target", target),
		row("Enc", encoderLabel(p.ctx.Encoder)),
		row("Preset", p.ctx.Quality),
		row("Out", out),
	}
}

func (p *processingState) rifeLines() []string {
	return []string{
		row("Stage", p.rifeStage),
		row("Prog", fmt.Sprintf("%s  %5.1f%%", render.ProgressBar(p.rifeProgress), p.rifeProgress)),
		row("ETA", formatDuration(p.rifeETA)),
		row("Speed", fmt.Sprintf("%.1f fps", p.rifeSpeed)),
		row("Trend", normalizedSpark(p.rifeFPSHist, 16)),
	}
}

func (p *processingState) encodeLines() []string {
	out := "—"
	if p.encodeSize > 0 {
		out = fmt.Sprintf("%s · %.1f Mb/s", humanBytes(p.encodeSize), p.encodeBitrate)
	}
	return []string{
		row("Stage", p.encodeStage),
		row("Prog", fmt.Sprintf("%s  %5.1f%%", render.ProgressBar(p.encodeProgress), p.encodeProgress)),
		row("ETA", formatDuration(p.encodeETA)),
		row("Out", out),
	}
}

func (p *processingState) systemLines() []string {
	return []string{
		row("CPU", fmt.Sprintf("%s  %5.1f%%", render.ProgressBar(p.cpuPct), p.cpuPct)),
		row("Mem", fmt.Sprintf("%s  %5.1f%%", render.ProgressBar(p.memPct), p.memPct)),
		row("GPU", fmt.Sprintf("%s  %d%%", render.MiniBar(p.gpuPct), int(p.gpuPct))),
	}
}

func (p *processingState) currentJob() *jobState {
	if p.currentJobID == "" {
		return nil
	}
	return p.jobs[p.currentJobID]
}

func (p *processingState) hardwareText() string {
	hw := "VideoToolbox"
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		hw = p.ctx.Caps.GPUName
	}
	if p.ctx.Encoder != "" {
		return hw + " · " + encoderLabel(p.ctx.Encoder)
	}
	return hw
}

// row formats a label/value pair with a fixed label column.
func row(label, value string) string {
	return fmt.Sprintf("%-6s %s", label, value)
}

// encoderLabel turns a config encoder value into its UI label.
func encoderLabel(v string) string {
	for _, c := range encoderChoices {
		if c.Value == v {
			return c.Label
		}
	}
	if v == "" {
		return "auto"
	}
	return v
}

// parseMediaSummary reads both "WxH, ffps, MM:SS" (probe events) and
// "WxH · ffps · MM:SS" (file picker previews).
func parseMediaSummary(s string) (w, h int, fps float64, dur time.Duration) {
	s = strings.ReplaceAll(s, "·", ",")
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		switch {
		case f == "":
		case w == 0 && strings.Contains(f, "x"):
			fmt.Sscanf(f, "%dx%d", &w, &h)
		case strings.HasSuffix(f, "fps"):
			fmt.Sscanf(strings.TrimSuffix(f, "fps"), "%f", &fps)
		case strings.Contains(f, ":"):
			dur = parseClock(f)
		}
	}
	return
}

func parseClock(s string) time.Duration {
	parts := strings.Split(strings.TrimSpace(s), ":")
	var h, m, sec int
	switch len(parts) {
	case 3:
		h, _ = strconv.Atoi(parts[0])
		m, _ = strconv.Atoi(parts[1])
		sec, _ = strconv.Atoi(parts[2])
	case 2:
		m, _ = strconv.Atoi(parts[0])
		sec, _ = strconv.Atoi(parts[1])
	default:
		return 0
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

// formatDuration prints MM:SS or HH:MM:SS; zero shows as --:--.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "--:--"
	}
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// normalizedSpark scales samples against their max, keeping the newest width.
func normalizedSpark(data []float64, width int) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) > width {
		data = data[len(data)-width:]
	}
	max := 0.0
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return render.Sparkline(data, width)
	}
	norm := make([]float64, len(data))
	for i, v := range data {
		norm[i] = v / max
	}
	return render.Sparkline(norm, width)
}

// nominalBitrateMbps is a rough per-preset output bitrate for the size estimate.
func nominalBitrateMbps(quality string) float64 {
	switch quality {
	case "quality":
		return 20
	case "speed":
		return 8
	default:
		return 14
	}
}

// outputPathFromMessage pulls the path off "path (dur → dur)".
func outputPathFromMessage(msg string) string {
	if i := strings.Index(msg, " ("); i > 0 {
		return msg[:i]
	}
	return msg
}

// expectedOutputPath mirrors pipeline.outputPathFor for display before we hear
// the real path back.
func expectedOutputPath(input, dir string, targetFPS float64) string {
	if input == "" {
		return ""
	}
	dir = expandHomeDir(dir)
	base := filepath.Base(input)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, fmt.Sprintf("%s_%.0ffps.mp4", stem, targetFPS))
}

func expandHomeDir(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

// systemStatsTick refreshes stats every two seconds.
func systemStatsTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return systemStatsTickMsg(t) })
}

func readSystemStatsCmd() tea.Cmd {
	return func() tea.Msg {
		cpu, mem, gpu := readSystemStats()
		return systemStatsMsg{CPU: cpu, Mem: mem, GPU: gpu}
	}
}

// readSystemStats samples whole-machine CPU/RSS percentages. GPU stays 0 until
// Phase 3 wires up Metal counters.
func readSystemStats() (cpu, mem, gpu float64) {
	return readCPUPercent(), readMemPercent(), readGPUPercent()
}

func readCPUPercent() float64 {
	out, err := exec.Command("ps", "-A", "-o", "%cpu=").Output()
	if err != nil {
		return 0
	}
	sum := 0.0
	for _, f := range strings.Fields(string(out)) {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			sum += v
		}
	}
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	pct := sum / float64(n)
	if pct > 100 {
		pct = 100
	}
	return pct
}

func readMemPercent() float64 {
	totalOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	total, err := strconv.ParseFloat(strings.TrimSpace(string(totalOut)), 64)
	if err != nil || total <= 0 {
		return 0
	}
	rssOut, err := exec.Command("ps", "-A", "-o", "rss=").Output()
	if err != nil {
		return 0
	}
	sumKB := 0.0
	for _, f := range strings.Fields(string(rssOut)) {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			sumKB += v
		}
	}
	pct := sumKB * 1024 / total * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

var gpuUtilRe = regexp.MustCompile(`"Device Utilization %"=(\d+)`)

func readGPUPercent() float64 {
	out, err := exec.Command("ioreg", "-r", "-c", "IOAccelerator", "-d", "1").Output()
	if err != nil {
		return 0
	}
	m := gpuUtilRe.FindSubmatch(out)
	if len(m) < 2 {
		return 0
	}
	v, _ := strconv.ParseFloat(string(m[1]), 64)
	return v
}
