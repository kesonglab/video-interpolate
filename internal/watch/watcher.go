package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// VideoHandler receives a new video file once it has stabilized.
type VideoHandler func(path string)

// Watcher watches a directory for new video files.
type Watcher struct {
	dir      string
	fsw      *fsnotify.Watcher
	handler  VideoHandler
	debounce time.Duration
	seen     map[string]bool // already handed off, avoid duplicates
}

var videoExtensions = []string{".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".ts", ".m4v", ".wmv"}

func New(dir string, handler VideoHandler) (*Watcher, error) {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		dir:      dir,
		fsw:      fsw,
		handler:  handler,
		debounce: 2 * time.Second, // let the file finish writing
		seen:     map[string]bool{},
	}, nil
}

// Run blocks until ctx is cancelled or the watcher fails.
// Existing video files in dir are handed off first.
func (w *Watcher) Run(ctx context.Context) error {
	defer w.fsw.Close()

	if err := w.fsw.Add(w.dir); err != nil {
		return err
	}

	// initial scan: anything already in the dir counts as "new"
	if entries, err := os.ReadDir(w.dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && w.isVideo(e.Name()) {
				full := filepath.Join(w.dir, e.Name())
				if !w.seen[full] {
					w.seen[full] = true
					w.handler(full)
				}
			}
		}
	}

	pending := map[string]time.Time{} // path -> first event time
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-w.fsw.Errors:
			// keep watching; surface the error and move on
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		case ev := <-w.fsw.Events:
			if !w.isVideo(ev.Name) || ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			pending[ev.Name] = time.Now()
		case <-tick.C:
			// hand off anything quiet for the full debounce window
			now := time.Now()
			for path, t := range pending {
				if now.Sub(t) < w.debounce {
					continue
				}
				delete(pending, path)
				if !w.seen[path] {
					w.seen[path] = true
					w.handler(path)
				}
			}
		}
	}
}

func (w *Watcher) isVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range videoExtensions {
		if ext == e {
			return true
		}
	}
	return false
}
