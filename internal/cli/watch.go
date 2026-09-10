package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/report"
	"github.com/kesonglab/video-interpolate/internal/system"
	"github.com/kesonglab/video-interpolate/internal/watch"
	"github.com/spf13/cobra"
)

// runWatch watches a directory and processes video files as they land.
func runWatch(cmd *cobra.Command, opts *runOptions, cfg *config.Config) error {
	w := cmd.OutOrStdout()

	events := make(chan pipeline.Event, 256)
	orch := pipeline.New(cfg, system.DetectCapabilities(), events)

	// the watcher enqueues files and pokes RunLoop so it doesn't busy-spin
	wakeup := make(chan struct{}, 1)
	wt, err := watch.New(expandWatchPath(opts.Watch), func(path string) {
		fmt.Fprintf(cmd.ErrOrStderr(), "[watch] new file: %s\n", path)
		orch.Enqueue([]string{path})
		select {
		case wakeup <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}

	rpt := report.NewReport(cfg)
	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		names := map[string]string{}
		for ev := range events {
			rpt.RecordEvent(ev)
			if opts.JSON {
				if line, err := report.EventLine(ev); err == nil {
					fmt.Fprintln(w, string(line))
				}
			} else {
				printWatchLine(w, ev, names, orch)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		cancel()
		fmt.Fprintln(cmd.ErrOrStderr(), "stopping watch...")
		<-sig
		os.Exit(130)
	}()

	watcherDone := make(chan struct{})
	go func() {
		if err := wt.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(cmd.ErrOrStderr(), "watch error: %v\n", err)
		}
		close(watcherDone)
	}()
	// if the watcher dies on its own (dir gone, fs error), stop the loop too
	go func() {
		<-watcherDone
		cancel()
	}()

	runErr := orch.RunLoop(ctx, wakeup)
	cancel() // stop the watcher
	<-watcherDone
	close(events)
	<-eventsDone

	rpt.Enrich(orch.Jobs())
	rpt.Finalize()
	if opts.JSON {
		return rpt.Write(w)
	}
	// job errors were already printed live; Ctrl+C exits cleanly
	_ = runErr
	return nil
}

// printWatchLine renders one event as a plain text line in watch mode.
func printWatchLine(w io.Writer, ev pipeline.Event, names map[string]string, orch *pipeline.Orchestrator) {
	name := names[ev.JobID]
	if name == "" {
		for _, j := range orch.Jobs() {
			if j.ID == ev.JobID {
				name = filepath.Base(j.InputPath)
				names[ev.JobID] = name
				break
			}
		}
	}
	if name == "" {
		name = ev.JobID
	}
	switch ev.Type {
	case pipeline.EventJobStarted:
		fmt.Fprintf(w, "[%s] started\n", name)
	case pipeline.EventJobCompleted:
		fmt.Fprintf(w, "[%s] done %s\n", name, ev.Message)
	case pipeline.EventJobFailed:
		fmt.Fprintf(w, "[%s] failed %v\n", name, ev.Error)
	case pipeline.EventJobSkipped:
		fmt.Fprintf(w, "[%s] skipped %s\n", name, ev.Message)
	}
}

func expandWatchPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return filepath.Clean(p)
}
