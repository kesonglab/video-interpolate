package system

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeRifeDir creates ~/rife-ncnn-vulkan/rife-ncnn-vulkan-<ver>-macos/rife-ncnn-vulkan
// in a temp dir and points os.UserHomeDir at it.
func fakeRifeDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("rife-ncnn-vulkan is unix-only")
	}
	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "rife-ncnn-vulkan", "rife-ncnn-vulkan-20221029-macos")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(binDir, "rife-ncnn-vulkan")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)
	return bin
}

func TestFindRifeBinary_NestedVersionedPath(t *testing.T) {
	want := fakeRifeDir(t)
	got, err := FindRifeBinary()
	if err != nil {
		t.Fatalf("FindRifeBinary: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindRifeBinary_NotInstalled(t *testing.T) {
	// point HOME at empty dir so the recursive search finds nothing
	t.Setenv("HOME", t.TempDir())
	if _, err := FindRifeBinary(); err == nil {
		t.Fatal("expected error when rife is not installed")
	}
}

func TestCheckDependencies_FindsRifeInNestedPath(t *testing.T) {
	_ = fakeRifeDir(t)
	deps := CheckDependencies()
	var rife *Dependency
	for i := range deps {
		if deps[i].Name == "rife-ncnn-vulkan" {
			rife = &deps[i]
			break
		}
	}
	if rife == nil {
		t.Fatal("rife-ncnn-vulkan missing from CheckDependencies output")
	}
	if !rife.OK {
		t.Fatalf("rife should be OK, got %#v", *rife)
	}
	if rife.Path == "" {
		t.Fatal("rife path should be set when OK")
	}
}
