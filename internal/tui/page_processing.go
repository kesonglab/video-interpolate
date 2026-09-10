package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

const fpsHistory = 30

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
	batchStart   time.Time

	rifeFPSHist []float64

	popup        Popup
	sp           spinner.Model
	rifeProgress float64
	rifeDone     bool
	rifeStage    string
	rifeETA      time.Duration
	rifeSpeed    float64

	encodeProgress float64
	encodeStage    string
	encodeETA      time.Duration
	encodeSize     int64

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
		sp:        newSpinner(),
	}
}

// newSpinner builds the queen dot spinner tinted blue.
func newSpinner() spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(render.ColorBlue)),
	)
}

func (p *processingState) Init() tea.Cmd {
	if p.ctx.SourceFPS == 0 && len(p.ctx.FileInfo) > 0 {
		if _, _, fps, _ := parseMediaSummary(p.ctx.FileInfo[0]); fps > 0 {
			p.ctx.SourceFPS = fps
			p.sourceFPS = fps
		}
	}
	return tea.Batch(waitForEvent(p.ctx.EventsCh), p.sp.Tick)
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
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.sp, cmd = p.sp.Update(msg)
		return p, tea.Batch(cmd, waitForEvent(p.ctx.EventsCh))
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
		p.popup = NewPopup("quit", "Quit", "Quit after the current file?", []string{"Cancel", "Quit"}, render.Yellow).Open()
	case "c":
		p.popup = NewPopup("cancel", "Cancel job", "Cancel the current job?", []string{"No", "Yes"}, render.Red).Open()
	case "n":
		p.popup = NewPopup("skip", "Skip file", "Skip the current file?", []string{"No", "Yes"}, render.Yellow).Open()
	case "p":
		// TODO: pause/resume needs orchestrator support
	}
	if key.Code == tea.KeyEsc {
		p.popup = NewPopup("quit", "Quit", "Quit after the current file?", []string{"Cancel", "Quit"}, render.Yellow).Open()
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
		p.rifeDone = false
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
	if p.batchStart.IsZero() {
		p.batchStart = time.Now()
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
		p.rifeDone = true
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
		bitrate := nominalBitrateMbps(p.ctx.Quality)
		outDur := p.sourceDur * time.Duration(p.ctx.Multiplier)
		total := int64(bitrate / 8 * 1e6 * outDur.Seconds())
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
	if width < 60 {
		width = 80
	}

	banner := components.Banner(appTitle(), authorLine, width)
	var rows []string
	for _, id := range p.jobOrder {
		if j := p.jobs[id]; j != nil {
			rows = append(rows, p.jobRow(j))
		}
	}

	batch := p.batchStats()
	batchBar := render.ProgressBar(float64(batch.pct))
	batchLine := fmt.Sprintf("  %s  %s  %d%%  (%d/%d)",
		render.Dim.Render("批量进度"), batchBar, batch.pct, batch.done, batch.total)
	batchLine += "\n  " + render.Dim.Render("  elapsed "+formatDuration(batch.elapsed)+
		" | ETA "+formatDuration(batch.eta)+" | press q to abort")

	body := banner + "\n\n" +
		strings.Join(rows, "\n") + "\n\n" +
		batchLine + "\n\n" +
		components.KeyHint([]components.HintPair{
			{Key: "c", Desc: "cancel current"},
			{Key: "n", Desc: "skip"},
			{Key: "p", Desc: "pause"},
			{Key: "q", Desc: "quit"},
		})

	if p.popup.Active() {
		return tea.NewView(p.popup.View(body, width, p.ctx.Height))
	}
	return tea.NewView(body)
}

// jobRow renders one job per queen's taskRow: path line + indented info.
func (p *processingState) jobRow(j *jobState) string {
	switch j.Status {
	case "pending":
		return "  " + render.Dim.Render("⏳") + " " + shortPath(j.InputPath) +
			"\n      " + render.Dim.Render("waiting")
	case "active":
		sp := p.sp.View()
		pct := p.curPercent()
		dl := p.rifeStage
		if p.encodeSize > 0 {
			dl = humanBytes(p.encodeSize)
		}
		speed := fmt.Sprintf("%.1f fps", p.rifeSpeed)
		if p.rifeSpeed <= 0 {
			speed = "—"
		}
		eta := formatDuration(p.curETA())
		info := fmt.Sprintf("%s %5.1f%% %-11s | %s %-11s | %s %8s",
			render.ProgressBar(pct),
			pct,
			render.Gray.Render(render.PadW(dl, 11)),
			"speed",
			render.Gray.Render(render.PadW(speed, 11)),
			"eta",
			render.Gray.Render(render.PadW(eta, 8)),
		)
		if trend := sparkline(p.rifeFPSHist, 8); trend != "" {
			info += "  " + render.Dim.Render(trend)
		}
		return "  " + sp + " " + shortPath(j.InputPath) + "\n      " + info
	case "done":
		return render.StatusDone(shortPath(j.OutputPath))
	case "failed":
		line := render.StatusFailed(shortPath(j.InputPath))
		if j.Error != "" {
			line += "\n      " + render.Red.Render(j.Error)
		}
		return line
	default: // skipped
		return render.Dim.Render("⚠") + " " + shortPath(j.InputPath) +
			"\n      " + render.Dim.Render("skipped")
	}
}

// curPercent is the progress of whatever stage is live.
func (p *processingState) curPercent() float64 {
	if p.rifeDone || p.encodeProgress > 0 {
		return p.encodeProgress
	}
	return p.rifeProgress
}

// curETA is the ETA of whatever stage is live.
func (p *processingState) curETA() time.Duration {
	if p.rifeDone || p.encodeProgress > 0 {
		return p.encodeETA
	}
	return p.rifeETA
}

type batchStats struct {
	done    int
	total   int
	pct     int
	elapsed time.Duration
	eta     time.Duration
}

func (p *processingState) batchStats() batchStats {
	s := batchStats{total: len(p.jobOrder)}
	for _, id := range p.jobOrder {
		if j := p.jobs[id]; j != nil && j.Status != "pending" && j.Status != "active" {
			s.done++
		}
	}
	if s.total > 0 {
		s.pct = s.done * 100 / s.total
	}
	if !p.batchStart.IsZero() {
		s.elapsed = time.Since(p.batchStart)
		if s.done > 0 && s.total > s.done {
			s.eta = time.Duration(float64(s.elapsed) / float64(s.done) * float64(s.total-s.done))
		}
	}
	return s
}

func (p *processingState) currentJob() *jobState {
	if p.currentJobID == "" {
		return nil
	}
	return p.jobs[p.currentJobID]
}

// shortPath keeps the path readable, queen shortURL style.
func shortPath(p string) string {
	if len(p) <= 52 {
		return p
	}
	// keep the tail (filename) intact
	base := filepath.Base(p)
	tail := len(base)
	if tail > 30 {
		tail = 30
	}
	return p[:52-tail] + "…" + base[len(base)-tail:]
}

// sparkline draws the last `width` fps samples as block glyphs.
func sparkline(samples []float64, width int) string {
	if len(samples) == 0 {
		return ""
	}
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}
	lo, hi := samples[0], samples[0]
	for _, v := range samples {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range samples {
		idx := 0
		if hi > lo {
			idx = int((v - lo) / (hi - lo) * float64(len(blocks)-1))
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
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
