package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/report"
	"github.com/kesonglab/video-interpolate/internal/system"
	"github.com/spf13/cobra"
)

type runOptions struct {
	Multiplier    string
	Encoder       string
	Quality       string
	Output        string
	GPU           int
	DryRun        bool
	Watch         string
	JSON          bool
	NoTUI         bool
	SkipExisting  bool
	ChunkSize     int
	NoChunk       bool
	Verbose       bool
	Quiet         bool
	Jobs          int
	Format        string
	FormatQuality int
}

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true,
	".webm": true, ".flv": true, ".ts": true, ".m4v": true, ".wmv": true,
}

func newRunCmd() *cobra.Command {
	opts := &runOptions{}
	cmd := &cobra.Command{
		Use:   "run <files...>",
		Short: "Run interpolation on given files",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipeline(cmd, opts, args)
		},
	}
	cmd.Flags().StringVarP(&opts.Multiplier, "multiplier", "m", "2x", "frame rate multiplier (2x/4x/8x)")
	cmd.Flags().StringVarP(&opts.Encoder, "encoder", "e", "auto", "encoder (auto/hevc_videotoolbox/h264_videotoolbox/libx264/libx265)")
	cmd.Flags().StringVarP(&opts.Quality, "quality", "q", "balanced", "quality preset (quality/balanced/speed)")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "output directory")
	cmd.Flags().IntVarP(&opts.GPU, "gpu", "g", 0, "GPU index (-1 for CPU)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview without running")
	cmd.Flags().StringVarP(&opts.Watch, "watch", "w", "", "watch directory for new video files")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit machine-readable JSON report (disables TUI)")
	cmd.Flags().BoolVar(&opts.SkipExisting, "skip-existing", true, "skip if output exists")
	cmd.Flags().IntVar(&opts.ChunkSize, "chunk-size", 3000, "frames per chunk (0 = no chunking)")
	cmd.Flags().BoolVar(&opts.NoChunk, "no-chunk", false, "disable chunking entirely")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "verbose logging")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false, "suppress non-error output")
	cmd.Flags().IntVarP(&opts.Jobs, "jobs", "j", 1, "parallel video jobs")
	cmd.Flags().StringVar(&opts.Format, "format", "jpg", "intermediate frame format (jpg/png)")
	cmd.Flags().IntVar(&opts.FormatQuality, "jpg-quality", 2, "jpeg quality (1-31, lower is better)")
	return cmd
}

func runPipeline(cmd *cobra.Command, opts *runOptions, args []string) error {
	multiplier, err := parseMultiplier(opts.Multiplier)
	if err != nil {
		return err
	}

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	// only flags the user actually set override the config file
	f := cmd.Flags()
	overrides := map[string]any{}
	if f.Changed("multiplier") {
		overrides["multiplier"] = multiplier
	}
	if f.Changed("encoder") {
		overrides["encoder"] = opts.Encoder
	}
	if f.Changed("quality") {
		overrides["quality"] = opts.Quality
	}
	if f.Changed("output") {
		overrides["output_dir"] = opts.Output
	}
	if f.Changed("gpu") {
		overrides["gpu"] = opts.GPU
	}
	if f.Changed("skip-existing") {
		overrides["skip_existing"] = opts.SkipExisting
	}
	if f.Changed("format") {
		overrides["format"] = opts.Format
	}
	if f.Changed("jpg-quality") {
		overrides["jpg_quality"] = opts.FormatQuality
	}
	if f.Changed("chunk-size") {
		overrides["chunk_size"] = opts.ChunkSize
	}
	if opts.NoChunk {
		overrides["chunk_size"] = 0
	}
	if f.Changed("watch") {
		overrides["watch_dir"] = opts.Watch
	}
	if f.Changed("jobs") {
		overrides["jobs"] = opts.Jobs
	}
	cfg.Merge(overrides)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%v\nrun 'vif run --help' for usage", err)
	}

	// JSON output is for scripts and pipes, never the interactive TUI
	if opts.JSON {
		opts.NoTUI = true
	}

	if opts.Watch != "" {
		return runWatch(cmd, opts, cfg)
	}

	paths, err := expandPaths(args)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no video files given")
	}

	if opts.Jobs > 1 {
		fmt.Fprintln(cmd.ErrOrStderr(), render.Warn.Render(fmt.Sprintf("note: %d jobs requested, running serially for now", opts.Jobs)))
	}

	caps := system.DetectCapabilities()
	events := make(chan pipeline.Event, 256)
	o := pipeline.New(cfg, caps, events)
	o.Enqueue(paths)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if opts.DryRun {
		lines, err := o.DryRun(ctx)
		for _, l := range lines {
			fmt.Fprintln(cmd.OutOrStdout(), l)
		}
		return err
	}

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		o.Cancel()
		cancel()
		fmt.Fprintln(cmd.ErrOrStderr(), render.Warn.Render("interrupting... press Ctrl+C again to quit"))
		<-sig
		os.Exit(130)
	}()

	if opts.JSON {
		rpt := report.NewReport(cfg)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for ev := range events {
				rpt.RecordEvent(ev)
			}
		}()
		err := o.Run(ctx)
		close(events)
		<-done
		rpt.Enrich(o.Jobs())
		rpt.Finalize()
		if werr := rpt.Write(cmd.OutOrStdout()); werr != nil {
			return werr
		}
		return err
	}

	jobs := o.Jobs()
	idx := make(map[string]int, len(jobs))
	base := make(map[string]string, len(jobs))
	for i, j := range jobs {
		idx[j.ID] = i
		base[j.ID] = filepath.Base(j.InputPath)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		printEvents(cmd.OutOrStdout(), events, idx, base)
	}()
	err = o.Run(ctx)
	close(events)
	<-done
	return err
}

func parseMultiplier(s string) (int, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "x"))
	n, err := strconv.Atoi(s)
	if err != nil || (n != 2 && n != 4 && n != 8) {
		return 0, fmt.Errorf("invalid multiplier %q, want 2x, 4x or 8x", s)
	}
	return n, nil
}

// printEvents renders job progress as plain text lines.
func printEvents(w io.Writer, events <-chan pipeline.Event, idx map[string]int, base map[string]string) {
	var cur pipeline.Stage
	for ev := range events {
		switch ev.Type {
		case pipeline.EventJobStarted:
			if cur != "" {
				fmt.Fprintln(w)
				cur = ""
			}
			fmt.Fprintf(w, "[%d/%d] %s\n", idx[ev.JobID]+1, len(idx), base[ev.JobID])
		case pipeline.EventStageChange:
			if ev.Stage == pipeline.StageProbing {
				fmt.Fprintf(w, "  %s %s\n", render.Primary.Render("[probe]"), ev.Message)
			} else if cur != "" {
				fmt.Fprintln(w)
				cur = ""
			}
		case pipeline.EventProgress:
			if ev.Stage != cur {
				if cur != "" {
					fmt.Fprintln(w)
				}
				cur = ev.Stage
				fmt.Fprintf(w, "  %s ", stageLabel(ev.Stage))
			}
			label := stageLabel(ev.Stage)
			if ev.ChunkTotal > 1 {
				label = fmt.Sprintf("%s %d/%d", label, ev.ChunkCurrent, ev.ChunkTotal)
			}
			line := fmt.Sprintf("\r  %s %s %3.0f%%", label, render.ProgressBar(ev.Progress), ev.Progress)
			if ev.FPS > 0 {
				line += fmt.Sprintf("  %.1f fps", ev.FPS)
			}
			if ev.ETA > 0 {
				line += "  ETA " + fmtETA(ev.ETA)
			}
			fmt.Fprint(w, line+" ")
		case pipeline.EventJobCompleted:
			if cur != "" {
				fmt.Fprintln(w)
				cur = ""
			}
			fmt.Fprintf(w, "  %s %s\n", render.OK.Render("✓"), ev.Message)
		case pipeline.EventJobSkipped:
			if cur != "" {
				fmt.Fprintln(w)
				cur = ""
			}
			fmt.Fprintf(w, "  %s %s\n", render.Warn.Render("⚠"), ev.Message)
		case pipeline.EventJobFailed:
			if cur != "" {
				fmt.Fprintln(w)
				cur = ""
			}
			fmt.Fprintf(w, "  %s %v\n", render.Danger.Render("✗"), ev.Error)
		}
	}
}

func stageLabel(s pipeline.Stage) string {
	switch s {
	case pipeline.StageProbing:
		return "[probe]"
	case pipeline.StageDecoding:
		return "[decode]"
	case pipeline.StageInterpolating:
		return "[rife]"
	case pipeline.StageEncoding:
		return "[encode]"
	}
	return "[" + string(s) + "]"
}

func fmtETA(d time.Duration) string {
	if d <= 0 {
		return "--:--"
	}
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// expandPaths turns args into a deduped list of video files, mirroring the
// original bash script: files, directories, space-split drag-and-drop, "-" for clipboard.
func expandPaths(args []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, arg := range args {
		if arg == "-" {
			lines, err := clipboardLines()
			if err != nil {
				return nil, fmt.Errorf("read clipboard: %w", err)
			}
			for _, l := range lines {
				expandOne(l, add)
			}
			continue
		}
		expandOne(arg, add)
	}
	return out, nil
}

func clipboardLines() ([]string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func expandOne(line string, add func(string)) {
	unesc := strings.ReplaceAll(line, `\ `, " ")
	if fi, err := os.Stat(unesc); err == nil && !fi.IsDir() {
		add(unesc)
		return
	}
	if fi, err := os.Stat(unesc); err == nil && fi.IsDir() {
		entries, err := os.ReadDir(unesc)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && videoExts[strings.ToLower(filepath.Ext(e.Name()))] {
				add(filepath.Join(unesc, e.Name()))
			}
		}
		return
	}
	// probably a drag-and-drop with escaped spaces: split, then re-join until a file matches
	parts := strings.Fields(line)
	for i := 0; i < len(parts); {
		cand := parts[i]
		j := i + 1
		found := false
		for {
			candU := strings.ReplaceAll(cand, `\ `, " ")
			if fi, err := os.Stat(candU); err == nil && !fi.IsDir() {
				add(candU)
				i = j
				found = true
				break
			}
			if j >= len(parts) {
				break
			}
			cand = cand + " " + parts[j]
			j++
		}
		if !found {
			i++
		}
	}
}
