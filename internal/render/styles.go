package render

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// queen palette: five colors, nothing else.
var (
	ColorGreen  = lipgloss.Color("#00ff87")
	ColorBlue   = lipgloss.Color("#00a6ff")
	ColorRed    = lipgloss.Color("#ff4d4f")
	ColorYellow = lipgloss.Color("#ffcc00")
	ColorGray   = lipgloss.Color("#888888")
)

var (
	Title  = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	Accent = lipgloss.NewStyle().Foreground(ColorBlue).Bold(true)
	Gray   = lipgloss.NewStyle().Foreground(ColorGray)
	Red    = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
	Yellow = lipgloss.NewStyle().Foreground(ColorYellow)
	Green  = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	Sel    = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	Dim    = lipgloss.NewStyle().Foreground(ColorGray)
	Sep    = lipgloss.NewStyle().Foreground(ColorGray).Faint(true)
	Box    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBlue).Padding(0, 1)

	// aliases so CLI/doctor keep compiling
	Primary = Accent
	Subtle  = Dim
	Warn    = Yellow
	Danger  = Red
	OK      = Green
	Line    = Sep
)

// queen icon set
const (
	IconSpinner = "⠋"
	IconDone    = "✓"
	IconFailed  = "✗"
	IconPending = "⏳"
	IconInfo    = "ℹ"
	IconCursor  = "▶"
	IconCrown   = "👑"
)

// Banner 返回顶部三行 banner：分隔条 + 标题+作者 + 分隔条
func Banner(title, author string, width int) string {
	sep := Sep.Render(strings.Repeat("━", max(width, 50)))
	head := Title.Render("  "+IconCrown+" "+title) + "  " + Gray.Render(author)
	return sep + "\n" + head + "\n" + sep
}

// MenuRow 单行菜单：游标 / [N] 数字键 / label / 描述
func MenuRow(cursor, idx int, key, label, desc string) string {
	marker := "  "
	if cursor == idx {
		marker = Sel.Render(IconCursor + " ")
	}
	k := Accent.Render("[" + key + "]")
	l := label
	if cursor == idx {
		l = Sel.Render(label)
	}
	return marker + k + " " + l + "  " + Dim.Render(desc)
}

// SettingRow 设置项行：游标 / label : value
func SettingRow(cursor, idx int, label, value string) string {
	marker := "  "
	if cursor == idx {
		marker = Sel.Render(IconCursor + " ")
	}
	l := label
	if cursor == idx {
		l = Sel.Render(label)
	}
	return marker + l + " : " + Accent.Render(value)
}

// SectionTitle 返回 "  ── title ──"
func SectionTitle(title string) string {
	return Title.Render("  ── " + title + " ──")
}

// Toast 黄色 ℹ 前缀提示
func Toast(msg string) string {
	return Yellow.Render("  " + IconInfo + " " + msg)
}

// StatusDone 绿色 ✓ 前缀
func StatusDone(s string) string { return "  " + Green.Render(IconDone+" ") + s }

// StatusFailed 红色 ✗ 前缀
func StatusFailed(s string) string { return "  " + Red.Render(IconFailed+" ") + s }

// StatusPending 灰色 ⏳ 前缀
func StatusPending(s string) string { return "  " + Dim.Render(IconPending+" ") + s }

// ProgressBar 24 宽蓝色进度条
func ProgressBar(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	const width = 24
	filled := int(pct / 100 * float64(width))
	return Accent.Render(strings.Repeat("█", filled) + strings.Repeat("░", width-filled))
}

// PadW 按显示宽度补齐到 n 列，避免数值变化时文字跳动
func PadW(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// RenderBar 纯文本进度条，不染色，给内部字符串拼接用
func RenderBar(pct float64, width int) string {
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	filled := int(pct / 100 * float64(width))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
