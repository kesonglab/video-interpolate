package render

import (
	"flag"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2/compat"
)

// -update rewrites golden files instead of comparing.
var update = flag.Bool("update", false, "update golden files")

// lipgloss v2 always emits ANSI colors, so strip them and compare plain text.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestPalette(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"title", Title.GetForeground(), ColorGreen},
		{"accent", Accent.GetForeground(), ColorBlue},
		{"red", Red.GetForeground(), ColorRed},
		{"yellow", Yellow.GetForeground(), ColorYellow},
		{"gray", Gray.GetForeground(), ColorGray},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s foreground = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestAdaptiveColors_LightDark(t *testing.T) {
	cases := []struct {
		name string
		c    compat.AdaptiveColor
	}{
		{"green", ColorGreen},
		{"blue", ColorBlue},
		{"red", ColorRed},
		{"yellow", ColorYellow},
		{"gray", ColorGray},
	}
	for _, tc := range cases {
		if tc.c.Light == nil {
			t.Errorf("%s: light variant empty", tc.name)
		}
		if tc.c.Dark == nil {
			t.Errorf("%s: dark variant empty", tc.name)
		}
		if tc.c.Light == tc.c.Dark {
			t.Errorf("%s: light and dark are identical (%v) — adaptive theme wouldn't change anything", tc.name, tc.c.Light)
		}
	}
}

func TestAdaptiveColors_BackgroundCheck(t *testing.T) {
	// light variants need to be dark enough to read on a white background
	cases := []struct {
		name string
		c    compat.AdaptiveColor
		max  float64
	}{
		{"green", ColorGreen, 0.5},
		{"red", ColorRed, 0.5},
		{"yellow", ColorYellow, 0.5},
		{"blue", ColorBlue, 0.4},
	}
	for _, tc := range cases {
		if l := wcagLuminance(tc.c.Light); l >= tc.max {
			t.Errorf("%s: light luminance %.3f, want < %.1f (too light for white bg)", tc.name, l, tc.max)
		}
	}
}

// wcagLuminance is relative luminance (0-1) per WCAG 2.x.
func wcagLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 65535.0
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

func TestProgressBar(t *testing.T) {
	cases := []struct {
		pct    float64
		filled int
	}{
		{0, 0},
		{50, 12},
		{100, 24},
		{150, 24},
		{-5, 0},
	}
	for _, c := range cases {
		got := stripANSI(ProgressBar(c.pct))
		if n := len([]rune(got)); n != 24 {
			t.Errorf("pct %v width = %d, want 24", c.pct, n)
		}
		if n := strings.Count(got, "█"); n != c.filled {
			t.Errorf("pct %v filled = %d, want %d (%q)", c.pct, n, c.filled, got)
		}
	}
}

func TestRenderBar(t *testing.T) {
	if got := RenderBar(50, 4); got != "██░░" {
		t.Errorf("RenderBar = %q, want %q", got, "██░░")
	}
	if got := RenderBar(150, 4); got != "████" {
		t.Errorf("RenderBar clamp high = %q", got)
	}
	if got := RenderBar(-10, 4); got != "░░░░" {
		t.Errorf("RenderBar clamp low = %q", got)
	}
}

func TestPadW(t *testing.T) {
	if got := PadW("ab", 5); got != "ab   " {
		t.Errorf("PadW = %q, want %q", got, "ab   ")
	}
	if got := PadW("abcdef", 3); got != "abcdef" {
		t.Errorf("PadW should not truncate: %q", got)
	}
}

func TestBanner(t *testing.T) {
	got := stripANSI(Banner("vif v0.1.0", "author", 12))
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("banner lines = %d, want 3:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[1], "👑 vif v0.1.0") {
		t.Errorf("banner title missing crown: %q", lines[1])
	}
	if !strings.Contains(lines[1], "author") {
		t.Errorf("banner author missing: %q", lines[1])
	}
}

func TestHelpers(t *testing.T) {
	if got := stripANSI(SectionTitle("Setup")); got != "  ── Setup ──" {
		t.Errorf("SectionTitle = %q", got)
	}
	if got := stripANSI(Toast("hi")); got != "  ℹ hi" {
		t.Errorf("Toast = %q", got)
	}
	if got := stripANSI(MenuRow(0, 0, "1", "x2", "48 fps")); !strings.HasPrefix(got, "▶ [1] x2") {
		t.Errorf("MenuRow = %q", got)
	}
	if got := stripANSI(MenuRow(-1, 0, "1", "x2", "48 fps")); !strings.HasPrefix(got, "  [1] x2") {
		t.Errorf("MenuRow unselected = %q", got)
	}
	if got := stripANSI(SettingRow(-1, 0, "Label", "value")); got != "  Label : value" {
		t.Errorf("SettingRow = %q", got)
	}
	if got := stripANSI(StatusDone("done")); got != "  ✓ done" {
		t.Errorf("StatusDone = %q", got)
	}
	if got := stripANSI(StatusFailed("bad")); got != "  ✗ bad" {
		t.Errorf("StatusFailed = %q", got)
	}
	if got := stripANSI(StatusPending("wait")); got != "  ⏳ wait" {
		t.Errorf("StatusPending = %q", got)
	}
}

func TestGoldenProgressBar(t *testing.T) {
	cases := []struct {
		name string
		pct  float64
	}{
		{"progressbar_0", 0},
		{"progressbar_50", 50},
		{"progressbar_100", 100},
	}
	for _, c := range cases {
		path := filepath.Join("testdata", "golden", c.name+".txt")
		got := stripANSI(ProgressBar(c.pct))
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run go test -update)", path, err)
		}
		if want := strings.TrimRight(string(raw), "\n"); got != want {
			t.Errorf("%s = %q, want %q", c.name, got, want)
		}
	}
}
