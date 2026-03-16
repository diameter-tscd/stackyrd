package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SimpleRenderer provides non-interactive styled console output
// for environments that don't support full TUI
type SimpleRenderer struct {
	width int
}

// NewSimpleRenderer creates a new simple renderer
func NewSimpleRenderer() *SimpleRenderer {
	return &SimpleRenderer{width: 60}
}

// PrintBanner prints a styled banner
func (r *SimpleRenderer) PrintBanner(text string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8daea5"))
	fmt.Println(style.Render(text))
}

// PrintHeader prints a styled header
func (r *SimpleRenderer) PrintHeader(appName, version, env string) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8daea5"))

	subStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8daea5")).
		Italic(true)

	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("✨ %s ✨", appName)))
	fmt.Println(subStyle.Render(fmt.Sprintf("v%s • %s environment", version, env)))
	fmt.Println()
}

// PrintDivider prints a styled divider line
func (r *SimpleRenderer) PrintDivider() {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262ff"))
	fmt.Println(style.Render(strings.Repeat("─", r.width)))
}

// PrintSection prints a section header
func (r *SimpleRenderer) PrintSection(title string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8daea5"))
	fmt.Println()
	fmt.Println(style.Render("◆ " + title))
	r.PrintDivider()
}

// PrintServiceStart prints a service starting message
func (r *SimpleRenderer) PrintServiceStart(name string) {
	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f5fac0ff")).
		Render("◐")

	nameStyle := lipgloss.NewStyle().
		Width(25).
		Foreground(lipgloss.Color("#F8F8F2"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f5fac0ff"))

	fmt.Printf("  %s %s %s %s\n", icon, nameStyle.Render(name), IconArrow, statusStyle.Render("starting..."))
}

// PrintServiceSuccess prints a service success message
func (r *SimpleRenderer) PrintServiceSuccess(name, message string) {
	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9af8b1ff")).
		Render("✓")

	nameStyle := lipgloss.NewStyle().
		Width(25).
		Foreground(lipgloss.Color("#F8F8F2"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9af8b1ff"))

	if message == "" {
		message = "ready"
	}
	fmt.Printf("  %s %s %s %s\n", icon, nameStyle.Render(name), IconArrow, statusStyle.Render(message))
}

// PrintServiceError prints a service error message
func (r *SimpleRenderer) PrintServiceError(name, message string) {
	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f67373ff")).
		Render("✗")

	nameStyle := lipgloss.NewStyle().
		Width(25).
		Foreground(lipgloss.Color("#F8F8F2"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f67373ff"))

	fmt.Printf("  %s %s %s %s\n", icon, nameStyle.Render(name), IconArrow, statusStyle.Render(message))
}

// PrintServiceSkipped prints a service skipped message
func (r *SimpleRenderer) PrintServiceSkipped(name string) {
	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262ff")).
		Render("○")

	nameStyle := lipgloss.NewStyle().
		Width(25).
		Foreground(lipgloss.Color("#626262ff")).
		Italic(true)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262ff")).
		Italic(true)

	fmt.Printf("  %s %s %s %s\n", icon, nameStyle.Render(name), IconArrow, statusStyle.Render("disabled"))
}

// PrintServerReady prints the server ready message
func (r *SimpleRenderer) PrintServerReady(port string, elapsed time.Duration) {
	fmt.Println()

	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#9af8b1ff"))

	highlightStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8daea5"))

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8daea5"))

	fmt.Println(successStyle.Render(fmt.Sprintf("🚀 Server ready at %s", highlightStyle.Render("http://localhost:"+port))))
	fmt.Println(infoStyle.Render(fmt.Sprintf("⚡ Started in %s", elapsed.Round(time.Millisecond))))
	fmt.Println()
}

// PrintProgressBar prints a progress bar
func (r *SimpleRenderer) PrintProgressBar(current, total int) {
	percent := float64(current) / float64(total) * 100
	bar := ProgressBar(percent, 40, true)
	fmt.Printf("\r  %s %d/%d", bar, current, total)
	if current == total {
		fmt.Println()
	}
}

// PrintInfo prints an info message
func (r *SimpleRenderer) PrintInfo(message string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8daea5"))
	fmt.Println(style.Render("ℹ " + message))
}

// PrintWarning prints a warning message
func (r *SimpleRenderer) PrintWarning(message string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f5fac0ff"))
	fmt.Println(style.Render("⚠ " + message))
}

// PrintError prints an error message
func (r *SimpleRenderer) PrintError(message string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f67373ff"))
	fmt.Println(style.Render("✗ " + message))
}

// PrintSuccess prints a success message
func (r *SimpleRenderer) PrintSuccess(message string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9af8b1ff"))
	fmt.Println(style.Render("✓ " + message))
}

// PrintBox prints content in a styled box
func (r *SimpleRenderer) PrintBox(title, content string) {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8daea5")).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#8daea5"))

	if title != "" {
		content = titleStyle.Render(title) + "\n" + content
	}

	fmt.Println(boxStyle.Render(content))
}

// AnimatedSpinner shows an animated spinner for a duration
func (r *SimpleRenderer) AnimatedSpinner(message string, duration time.Duration) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))

	start := time.Now()
	i := 0
	for time.Since(start) < duration {
		fmt.Printf("\r  %s %s", style.Render(frames[i%len(frames)]), msgStyle.Render(message))
		time.Sleep(80 * time.Millisecond)
		i++
	}
	fmt.Println()
}

// WaveAnimation prints a simple wave animation
func (r *SimpleRenderer) WaveAnimation(duration time.Duration) {
	waveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9af8b1ff"))

	start := time.Now()
	for time.Since(start) < duration {
		fmt.Printf("\r%s", waveStyle.Render("✨ Starting... ✨"))
		time.Sleep(200 * time.Millisecond)
		fmt.Printf("\r%s", waveStyle.Render("🌟 Starting... 🌟"))
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println()
}

// StartupAnimation runs a complete startup animation sequence
func (r *SimpleRenderer) StartupAnimation(cfg StartupConfig, services []ServiceInit) {
	startTime := time.Now()

	// Banner
	if cfg.Banner != "" {
		r.PrintBanner(cfg.Banner)
	}

	// Header
	r.PrintHeader(cfg.AppName, cfg.AppVersion, cfg.Env)

	// Boot animation
	r.WaveAnimation(500 * time.Millisecond)

	// Services section
	r.PrintSection("Boot Sequence")

	completed := 0
	for _, svc := range services {
		if !svc.Enabled {
			r.PrintServiceSkipped(svc.Name)
			completed++
			continue
		}

		r.PrintServiceStart(svc.Name)

		// Simulate or execute initialization
		var err error
		if svc.InitFunc != nil {
			err = svc.InitFunc()
		} else {
			time.Sleep(100 * time.Millisecond) // Brief delay for visual effect
		}

		// Clear the "starting" line and print result
		fmt.Print("\033[1A\033[2K") // Move up and clear line

		if err != nil {
			r.PrintServiceError(svc.Name, err.Error())
		} else {
			r.PrintServiceSuccess(svc.Name, "ready")
		}
		completed++
	}

	// Final message
	elapsed := time.Since(startTime)
	r.PrintServerReady(cfg.Port, elapsed)
}

// IsTUISupported checks if the terminal supports full TUI
func IsTUISupported() bool {
	// This is a simple check - in production you might want
	// to check for TERM environment variable, etc.
	return true
}

// RunStartup runs either the full TUI or simple startup based on terminal support
func RunStartup(cfg StartupConfig, services []ServiceInit) {
	if IsTUISupported() {
		// Try running Bubble Tea TUI
		_, err := RunBootSequence(cfg, services)
		if err != nil {
			// Fall back to simple renderer
			r := NewSimpleRenderer()
			r.StartupAnimation(cfg, services)
		}
	} else {
		// Use simple renderer
		r := NewSimpleRenderer()
		r.StartupAnimation(cfg, services)
	}
}
