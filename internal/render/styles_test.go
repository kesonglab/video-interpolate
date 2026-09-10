package render

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// lipgloss v2 always emits ANSI colors, so strip them and compare plain ASCII
// against golden files. NO_COLOR is ignored by this lipgloss version.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	cases := []struct {
		name string
		got  string
	}{
		{"progressbar_0", ProgressBar(0)},
		{"progressbar_50", ProgressBar(50)},
		{"progressbar_100", ProgressBar(100)},
		{"minibar_0", MiniBar(0)},
		{"minibar_50", MiniBar(50)},
		{"minibar_100", MiniBar(100)},
		{"sparkline", Sparkline([]float64{0.1, 0.5, 0.9, 1}, 4)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "golden", c.name+".txt"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got := stripANSI(c.got); got != string(want) {
				t.Errorf("got %q, want %q", got, string(want))
			}
		})
	}
}
