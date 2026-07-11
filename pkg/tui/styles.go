package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Fancy ASCII art banners for enhanced visual appeal
var (
	// Gradient colors for text effects
	GradientPink   = []string{"#8daea5", "#8daea5", "#8daea5", "#8daea5", "#8daea5"}
	GradientPurple = []string{"#BD93F9", "#9B59B6", "#8E44AD", "#6C3483", "#5B2C6F"}
	GradientCyan   = []string{"#8BE9FD", "#00D0FF", "#00B4D8", "#0096C7", "#0077B6"}
	GradientGreen  = []string{"#50FA7B", "#00FF7F", "#00FA9A", "#00CED1", "#20B2AA"}
)

// TextEffect applies a gradient effect to text
func TextEffect(text string, colors []string) string {
	if len(colors) == 0 || len(text) == 0 {
		return text
	}

	var result strings.Builder
	for i, char := range text {
		colorIdx := i % len(colors)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIdx]))
		result.WriteString(style.Render(string(char)))
	}
	return result.String()
}

// BoxStyles for different visual contexts
var (
	// Success box
	SuccessBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#50FA7B")).
		Foreground(lipgloss.Color("#50FA7B")).
		Padding(0, 2)

	// Warning box
	WarningBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#F1FA8C")).
		Foreground(lipgloss.Color("#F1FA8C")).
		Padding(0, 2)

	// Error box
	ErrorBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF5555")).
		Foreground(lipgloss.Color("#FF5555")).
		Padding(0, 2)

	// Info box
	InfoBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8BE9FD")).
		Foreground(lipgloss.Color("#8BE9FD")).
		Padding(0, 2)

	// Primary box with double border
	PrimaryBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#BD93F9")).
			Foreground(lipgloss.Color("#F8F8F2")).
			Padding(1, 2)
)

// Icons for consistent visual language
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

// DividerLine style for horizontal separators
var DividerLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))

// Divider creates a styled horizontal divider
func Divider(width int, char string) string {
	if char == "" {
		char = "─"
	}
	return DividerLine.Render(strings.Repeat(char, width))
}

// Header creates a styled header with decorations
func Header(text string) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF79C6")).
		Padding(0, 1)

	decorated := "◆ " + text + " ◆"
	return style.Render(decorated)
}

// SubHeader creates a styled subheader
func SubHeader(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8BE9FD")).
		Italic(true).
		Render(text)
}

// StatusBadge creates a colored status badge
func StatusBadge(status string) string {
	var style lipgloss.Style

	switch strings.ToLower(status) {
	case "success", "ok", "running", "active", "connected":
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("#50FA7B")).
			Foreground(lipgloss.Color("#282A36")).
			Padding(0, 1).
			Bold(true)
	case "error", "fail", "failed", "disconnected":
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF5555")).
			Foreground(lipgloss.Color("#F8F8F2")).
			Padding(0, 1).
			Bold(true)
	case "warning", "warn", "degraded":
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("#F1FA8C")).
			Foreground(lipgloss.Color("#282A36")).
			Padding(0, 1).
			Bold(true)
	case "pending", "loading", "starting":
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("#FFB86C")).
			Foreground(lipgloss.Color("#282A36")).
			Padding(0, 1).
			Bold(true)
	default:
		style = lipgloss.NewStyle().
			Background(lipgloss.Color("#6272A4")).
			Foreground(lipgloss.Color("#F8F8F2")).
			Padding(0, 1)
	}

	return style.Render(strings.ToUpper(status))
}

// KeyValue formats a key-value pair
func KeyValue(key, value string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8BE9FD")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2"))

	return keyStyle.Render(key+":") + " " + valueStyle.Render(value)
}

// Sidebar / main-panel styles for the opencode-style terminal dashboard
var (
	// Sidebar container: black background (matches the main panel), 4-cell padding
	sidebarStyle = lipgloss.NewStyle().
			Padding(0, 4).
			Align(lipgloss.Left)

	// Main panel container with 2-cell left/right padding
	mainPanelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Padding(0, 2).
			Align(lipgloss.Left)

	// Sidebar header (app name + version, brand accent)
	sidebarHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#8daea5"))

	// Sidebar section divider label (e.g. "─── Resources ───")
	sidebarSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272A4")).
				Bold(true)

	// Sidebar key label (e.g. "CPU:")
	sidebarLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D3DAD9"))

	// Sidebar plain value
	sidebarValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D3DAD9"))

	// Sidebar dim text
	sidebarDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	// Command input box — flat, opencode-style (no border)
	commandBoxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#37353E")).
			Foreground(lipgloss.Color("#D3DAD9")).
			Padding(0, 2)

	// Active command input box (focused)
	commandBoxActiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#37353E")).
				Foreground(lipgloss.Color("#D3DAD9")).
				Padding(0, 2)

	// ":" prompt in command input
	commandPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8daea5")).
				Bold(true)

	// Focus indicator for the current focus target
	focusIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8daea5")).
				Bold(true)
)

// ProgressBar creates a simple text-based progress bar
func ProgressBar(percent float64, width int, showPercent bool) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	filled := int((percent / 100.0) * float64(width))
	empty := width - filled

	var color string
	switch {
	case percent < 50:
		color = "#b0ffc4ff"
	case percent < 80:
		color = "#ffdab3ff"
	default:
		color = "#ffaeaeff"
	}

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#44475A"))

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	if showPercent {
		percentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		bar += " " + percentStyle.Render(fmt.Sprintf("%.0f%%", percent))
	}

	return bar
}
