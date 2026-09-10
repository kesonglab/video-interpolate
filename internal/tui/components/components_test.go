package components

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kesonglab/video-interpolate/internal/render"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// writeGolden rewrites golden files when UPDATE_GOLDEN=1 (test helper).
func writeGolden(t *testing.T, name, got string) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		return
	}
	dir := filepath.Join("testdata", "golden")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".golden"), []byte(stripANSI(got)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	cases := []struct {
		name string
		got  string
	}{
		{"banner", Banner("vif v0.1.0", "author", 20)},
		{"box", Box("hello", 20)},
		{"menu", Menu([]MenuItem{
			{Key: "1", Label: "x2", Description: "48 fps", Tag: "recommended", TagStyle: render.Green},
			{Key: "2", Label: "x4", Description: "96 fps"},
		}, 1, 40)},
		{"keyhint", KeyHint([]HintPair{{Key: "enter", Desc: "next"}, {Key: "esc", Desc: "back"}})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeGolden(t, c.name, c.got)
			want, err := os.ReadFile(filepath.Join("testdata", "golden", c.name+".golden"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got := stripANSI(c.got); got != string(want) {
				t.Errorf("got:\n%s\nwant:\n%s", got, string(want))
			}
		})
	}
}
