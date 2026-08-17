package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	GradientPink   = []string{"#8daea5", "#8daea5", "#8daea5", "#8daea5", "#8daea5"}
	GradientPurple = []string{"#BD93F9", "#9B59B6", "#8E44AD", "#6C3483", "#5B2C6F"}
	GradientCyan   = []string{"#8BE9FD", "#00D0FF", "#00B4D8", "#0096C7", "#0077B6"}
	GradientGreen  = []string{"#50FA7B", "#00FF7F", "#00FA9A", "#00CED1", "#20B2AA"}
)

func TextEffect(text string, colors []string) string {
	if len(colors) == 0 || len(text) == 0 {
		return text
	}
	var result strings.Builder
	runeIdx := 0
	for _, char := range text {
		colorIdx := runeIdx % len(colors)
		runeIdx++
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIdx]))
		result.WriteString(style.Render(string(char)))
	}
	return result.String()
}

// BoxStyle functions return fresh lipgloss.Style with current theme colors.
func SuccessBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(TC("success"))).Foreground(lipgloss.Color(TC("success"))).Padding(0, 2)
}
func WarningBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(TC("warning"))).Foreground(lipgloss.Color(TC("warning"))).Padding(0, 2)
}
func ErrorBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(TC("error"))).Foreground(lipgloss.Color(TC("error"))).Padding(0, 2)
}
func InfoBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(TC("secondary"))).Foreground(lipgloss.Color(TC("secondary"))).Padding(0, 2)
}
func PrimaryBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color(TC("primary"))).Foreground(lipgloss.Color(TC("text"))).Padding(1, 2)
}

const (
	IconSuccess  = "✓"
	IconError    = "✗"
	IconWarning  = "⚠"
	IconInfo     = "ℹ"
	IconLoading  = "◐"
	IconRocket   = "🚀"
	IconSparkle  = "✨"
	IconServer   = "⚡"
	IconDatabase = "💾"
	IconNetwork  = "🌐"
	IconClock    = "⏱"
	IconGear     = "⚙"
	IconCheck    = "✔"
	IconDot      = "●"
	IconCircle   = "○"
	IconArrow    = "→"
	IconPlay     = "▶"
	IconStop     = "■"
	IconPause    = "⏸"
	IconHeart    = "❤"
	IconStar     = "★"
	IconFire     = "🔥"
)

func DividerLine() lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim"))) }

func Divider(width int, char string) string {
	if char == "" {
		char = "─"
	}
	if width < 0 {
		width = 0
	}
	return DividerLine().Render(strings.Repeat(char, width))
}

func Header(text string) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(TC("secondary"))).Padding(0, 1)
	decorated := "◆ " + text + " ◆"
	return style.Render(decorated)
}

func SubHeader(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("secondary"))).Italic(true).Render(text)
}

func StatusBadge(status string) string {
	var style lipgloss.Style
	switch strings.ToLower(status) {
	case "success", "ok", "running", "active", "connected":
		style = lipgloss.NewStyle().Background(lipgloss.Color(TC("success"))).Foreground(lipgloss.Color(TC("text"))).Padding(0, 1).Bold(true)
	case "error", "fail", "failed", "disconnected":
		style = lipgloss.NewStyle().Background(lipgloss.Color(TC("error"))).Foreground(lipgloss.Color(TC("text"))).Padding(0, 1).Bold(true)
	case "warning", "warn", "degraded":
		style = lipgloss.NewStyle().Background(lipgloss.Color(TC("warning"))).Foreground(lipgloss.Color(TC("text"))).Padding(0, 1).Bold(true)
	case "pending", "loading", "starting":
		style = lipgloss.NewStyle().Background(lipgloss.Color(TC("secondary"))).Foreground(lipgloss.Color(TC("text"))).Padding(0, 1).Bold(true)
	default:
		style = lipgloss.NewStyle().Background(lipgloss.Color(TC("dim"))).Foreground(lipgloss.Color(TC("text"))).Padding(0, 1)
	}
	return style.Render(strings.ToUpper(status))
}

func KeyValue(key, value string) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(TC("secondary"))).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text")))
	return keyStyle.Render(key+":") + " " + valueStyle.Render(value)
}

// Sidebar / main-panel styles - functions create fresh styles with current theme colors
func sidebarStyle() lipgloss.Style { return lipgloss.NewStyle().Padding(0, 4).Align(lipgloss.Left) }
func mainPanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text"))).Padding(0, 4).Align(lipgloss.Left)
}
func sidebarHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(TC("primary")))
}
func sidebarSectionStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim"))).Bold(true)
}
func sidebarLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text")))
}
func sidebarValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text")))
}
func sidebarDimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim")))
}
func commandBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color("#37353E")).Foreground(lipgloss.Color(TC("text"))).Padding(0, 2)
}
func commandBoxActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color("#37353E")).Foreground(lipgloss.Color(TC("text"))).Padding(0, 2)
}
func commandPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary"))).Bold(true)
}
func focusIndicatorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary"))).Bold(true)
}

func ProgressBar(percent float64, width int, showPercent bool) string {
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	if width < 0 {
		width = 0
	}
	filled := int((percent / 100.0) * float64(width))
	empty := width - filled
	var color string
	switch {
	case percent < 50:
		color = TC("success")
	case percent < 80:
		color = TC("warning")
	default:
		color = TC("error")
	}
	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim")))
	bar := filledStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty))
	if showPercent {
		percentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		bar += " " + percentStyle.Render(fmt.Sprintf("%.0f%%", percent))
	}
	return bar
}
