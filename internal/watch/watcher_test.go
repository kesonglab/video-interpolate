package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startWatcher(t *testing.T, dir string, got chan<- string) context.CancelFunc {
	t.Helper()
	w, err := New(dir, func(path string) {
		select {
		case got <- path:
		default:
		}
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("watcher did not stop")
		}
	})
	return cancel
}

func TestWatcher_InitialScan(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.mp4", "b.mkv"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := make(chan string, 4)
	startWatcher(t, dir, got)

	// initial scan runs right away, no debounce
	for i := 0; i < 2; i++ {
		select {
		case p := <-got:
			if filepath.Base(p) != "a.mp4" && filepath.Base(p) != "b.mkv" {
				t.Errorf("unexpected path %s", p)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("handler not called for existing video %d", i)
		}
	}
	select {
	case p := <-got:
		t.Errorf("txt file handled: %s", p)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestWatcher_NewFile(t *testing.T) {
	dir := t.TempDir()
	got := make(chan string, 4)
	startWatcher(t, dir, got)

	newFile := filepath.Join(dir, "clip.mov")
	if err := os.WriteFile(newFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case p := <-got:
		if p != newFile {
			t.Errorf("got %s, want %s", p, newFile)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("new file not handled within 4s")
	}
}

func TestWatcher_NonVideo(t *testing.T) {
	dir := t.TempDir()
	got := make(chan string, 4)
	startWatcher(t, dir, got)

	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-got:
		t.Errorf("non-video handled: %s", p)
	case <-time.After(1 * time.Second):
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	got := make(chan string, 4)
	w, err := New(dir, func(string) { got <- "" })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
