package tui

import (
	"reflect"
	"testing"
)

func TestParsePastedPathsMultiLine(t *testing.T) {
	input := "/path/to/a.mp4\n/path/to/b.mov\n/path/with\\ space/c.mp4"
	got := parsePastedPaths(input)
	want := []string{"/path/to/a.mp4", "/path/to/b.mov", "/path/with space/c.mp4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePastedPaths = %#v, want %#v", got, want)
	}
}

func TestParsePastedPathsQuoted(t *testing.T) {
	input := `"/path/with space/a.mp4" '/another/path/b.mov'`
	got := parsePastedPaths(input)
	want := []string{"/path/with space/a.mp4", "/another/path/b.mov"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePastedPaths = %#v, want %#v", got, want)
	}
}

func TestParsePastedPathsDedup(t *testing.T) {
	got := parsePastedPaths("/a.mp4\n/a.mp4\n/a.mp4")
	if len(got) != 1 || got[0] != "/a.mp4" {
		t.Fatalf("parsePastedPaths = %#v, want one /a.mp4", got)
	}
}

func TestParsePastedPathsEmpty(t *testing.T) {
	if got := parsePastedPaths("  \n\t\n"); len(got) != 0 {
		t.Fatalf("parsePastedPaths = %#v, want empty", got)
	}
}

func TestIsVideoExt(t *testing.T) {
	yes := []string{"a.mp4", "B.MKV", "c.mov", "d.avi", "e.webm", "f.flv", "g.ts", "h.m4v", "i.wmv"}
	for _, name := range yes {
		if !isVideoExt(name) {
			t.Errorf("isVideoExt(%q) = false, want true", name)
		}
	}
	no := []string{"a.txt", "b.jpg", "c", "d.mp3", "e.MP4.bak"}
	for _, name := range no {
		if isVideoExt(name) {
			t.Errorf("isVideoExt(%q) = true, want false", name)
		}
	}
}
