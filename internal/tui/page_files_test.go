package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
)

// newTestFilePicker returns a page whose prober always succeeds, so tests
// don't need ffprobe.
func newTestFilePicker() (*filePickerPage, *SharedContext) {
	ctx := NewContext(config.Default(), nil)
	fp := NewFilePickerPage(ctx).(*filePickerPage)
	fp.probeFn = func(string) string { return "1920x1080 · 24fps · 00:00:01" }
	return fp, ctx
}

func touch(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFilePickerPasteAddsFiles(t *testing.T) {
	dir := t.TempDir()
	a := touch(t, dir, "a.mp4")
	b := touch(t, dir, "b.mov")

	fp, _ := newTestFilePicker()
	fp.Update(tea.PasteMsg{Content: a + "\n" + b})

	if len(fp.files) != 2 {
		t.Fatalf("files = %d, want 2", len(fp.files))
	}
	if fp.ti.Value() != "" {
		t.Fatalf("input not cleared: %q", fp.ti.Value())
	}
}

func TestFilePickerPasteDedups(t *testing.T) {
	dir := t.TempDir()
	a := touch(t, dir, "a.mp4")

	fp, _ := newTestFilePicker()
	fp.Update(tea.PasteMsg{Content: a + "\n" + a})

	if len(fp.files) != 1 {
		t.Fatalf("files = %d, want 1", len(fp.files))
	}
}

func TestFilePickerPasteExpandsDirectory(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "a.mp4")
	touch(t, dir, "b.mkv")
	touch(t, dir, "notes.txt")

	fp, _ := newTestFilePicker()
	fp.Update(tea.PasteMsg{Content: dir})

	if len(fp.files) != 2 {
		t.Fatalf("files = %d, want 2 videos", len(fp.files))
	}
}

func TestFilePickerPasteMissToasts(t *testing.T) {
	fp, ctx := newTestFilePicker()
	fp.Update(tea.PasteMsg{Content: "/nope/does-not-exist.mp4"})

	if len(fp.files) != 0 {
		t.Fatalf("files = %d, want 0", len(fp.files))
	}
	if ctx.Toast == "" {
		t.Fatal("expected a toast for the missed path")
	}
}

func TestFilePickerPasteNonVideoToasts(t *testing.T) {
	dir := t.TempDir()
	txt := touch(t, dir, "readme.txt")

	fp, ctx := newTestFilePicker()
	fp.Update(tea.PasteMsg{Content: txt})

	if len(fp.files) != 0 {
		t.Fatalf("files = %d, want 0", len(fp.files))
	}
	if ctx.Toast == "" {
		t.Fatal("expected a toast for the non-video file")
	}
}

// TestFilePickerDragThenEnter drives the real app: paste a path, then enter
// advances to the multiplier page.
func TestFilePickerDragThenEnter(t *testing.T) {
	a := New(config.Default(), nil)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	pumpNav(a, enterKey()) // welcome → files

	fp := a.pages[pageFiles].(*filePickerPage)
	fp.probeFn = func(string) string { return "1920x1080 · 24fps · 00:00:01" }
	src := touch(t, t.TempDir(), "clip.mp4")

	a.Update(tea.PasteMsg{Content: src})
	if len(fp.files) != 1 {
		t.Fatalf("files = %d, want 1", len(fp.files))
	}
	pumpNav(a, enterKey())
	if a.page != pageMultiplier {
		t.Fatalf("page = %d, want multiplier", a.page)
	}
}
