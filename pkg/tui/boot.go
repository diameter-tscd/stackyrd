package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServiceInitFunc is a function that initializes a service
// Returns an error if initialization fails
type ServiceInitFunc func() error

// ServiceInit represents a service to initialize
type ServiceInit struct {
	Name     string
	Enabled  bool
	InitFunc ServiceInitFunc
}

// BootModel is the Bubble Tea model for the boot sequence
type BootModel struct {
	spinner       spinner.Model
	initQueue     []ServiceInit
	results       []ServiceStatus
	current       int
	done          bool
	config        StartupConfig
	startTime     time.Time
	width         int
	phase         string // "starting", "initializing", "complete", "countdown", "error"
	animFrame     int
	countdown     int       // remaining seconds in countdown
	countdownTime time.Time // when countdown started
}

// Simple spinner frames
var bootFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// Boot styles
var (
	bootBannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8daea5"))

	bootSubStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)

	bootBoxBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(1, 2)

	bootCompleteStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#545454ff"))

	bootErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffaeaeff"))

	bootPhaseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#faffc7ff")).
			Bold(true)

	bootInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c7f5ffff"))

	bootSuccessIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b0ffc4ff")).
			Render("✓")

	bootErrorIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff9b9bff")).
			Render("✗")

	bootSkipIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Render("○")

	bootPendingIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Render("◦")
)

// Messages for boot model
type bootTickMsg time.Time
type bootDoneMsg struct{}

// NewBootModel creates a new boot model
func NewBootModel(cfg StartupConfig, initQueue []ServiceInit) BootModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle()

	results := make([]ServiceStatus, len(initQueue))
	for i, svc := range initQueue {
		status := "pending"
		if !svc.Enabled {
			status = "skipped"
		}
		results[i] = ServiceStatus{
			Name:   svc.Name,
			Status: status,
		}
	}

	return BootModel{
		spinner:   s,
		initQueue: initQueue,
		results:   results,
		config:    cfg,
		startTime: time.Now(),
		width:     100,
		phase:     "starting",
	}
}

func (m BootModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		bootTickCmd(),
	)
}

func bootTickCmd() tea.Cmd {
	return tea.Every(time.Millisecond*80, func(t time.Time) tea.Msg {
		return bootTickMsg(t)
	})
}

func (m BootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case bootTickMsg:
		m.animFrame = (m.animFrame + 1) % len(bootFrames)

		if m.phase == "starting" {
			// Brief intro animation
			if m.animFrame > 5 {
				m.phase = "initializing"
			}
			return m, tea.Batch(m.spinner.Tick, bootTickCmd())
		}

		if m.phase == "initializing" {
			// Find next pending service
			for m.current < len(m.initQueue) {
				if m.results[m.current].Status == "skipped" {
					m.current++
					continue
				}
				break
			}

			if m.current >= len(m.initQueue) {
				m.phase = "complete"
				m.done = true
				// Start countdown if configured
				if m.config.IdleSeconds > 0 {
					m.countdown = m.config.IdleSeconds
					m.countdownTime = time.Now()
					m.phase = "countdown"
					return m, tea.Batch(m.spinner.Tick, bootTickCmd())
				}
				return m, tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
					return bootDoneMsg{}
				})
			}

			// Initialize current service
			if m.results[m.current].Status == "pending" {
				m.results[m.current].Status = "loading"
				m.results[m.current].Message = "Initializing..."

				// Run initialization in background (simulated for now)
				svc := m.initQueue[m.current]
				if svc.InitFunc != nil {
					err := svc.InitFunc()
					if err != nil {
						m.results[m.current].Status = "error"
						m.results[m.current].Message = err.Error()
					} else {
						m.results[m.current].Status = "success"
						m.results[m.current].Message = "Ready"
					}
				} else {
					m.results[m.current].Status = "success"
					m.results[m.current].Message = "Ready"
				}
				m.current++
			}

			return m, tea.Batch(m.spinner.Tick, bootTickCmd())
		}

		if m.phase == "countdown" {
			// Update countdown based on elapsed time
			elapsed := int(time.Since(m.countdownTime).Seconds())
			m.countdown = m.config.IdleSeconds - elapsed

			if m.countdown <= 0 {
				return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
					return bootDoneMsg{}
				})
			}
			return m, tea.Batch(m.spinner.Tick, bootTickCmd())
		}

		if m.phase == "complete" || m.phase == "error" {
			return m, tea.Batch(m.spinner.Tick, bootTickCmd())
		}

	case bootDoneMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m BootModel) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Simple title
	title := fmt.Sprintf(" %s ", m.config.AppName)
	b.WriteString(bootBannerStyle.Bold(true).Render(title))
	b.WriteString("\n")

	// Version and env
	sub := fmt.Sprintf("v%s • %s environment", m.config.AppVersion, m.config.Env)
	b.WriteString(bootSubStyle.Render(sub))
	b.WriteString("\n\n")

	// Phase indicator
	phaseIcon := bootFrames[m.animFrame%len(bootFrames)]
	phaseText := ""
	switch m.phase {
	case "starting":
		phaseText = "Starting up..."
	case "initializing":
		phaseText = "Initializing services..."
	case "complete":
		phaseText = "Boot complete!"
		phaseIcon = "✓"
	case "countdown":
		phaseText = "Boot complete!"
		phaseIcon = "✓"
	case "error":
		phaseText = "Boot failed!"
		phaseIcon = "✗"
	}
	b.WriteString(fmt.Sprintf("%s %s\n\n", phaseIcon, bootPhaseStyle.Render(phaseText)))

	// Simple progress text
	completed := 0
	total := 0
	for _, r := range m.results {
		if r.Status != "skipped" {
			total++
		}
		if r.Status == "success" || r.Status == "error" {
			completed++
		}
	}
	if total > 0 {
		b.WriteString(fmt.Sprintf("Progress: %d/%d services\n\n", completed, total))
	}

	// Services list
	servicesContent := m.renderBootServices()
	b.WriteString(servicesContent)
	b.WriteString("\n")

	// Final message
	if m.done {
		elapsed := time.Since(m.startTime).Round(time.Millisecond)

		switch m.phase {
		case "complete":
			msg := fmt.Sprintf("\n Server ready at http://localhost:%s", m.config.Port)
			b.WriteString(bootCompleteStyle.Render(msg))
			b.WriteString("\n")
			b.WriteString(bootInfoStyle.Render(fmt.Sprintf(" Started in %s", elapsed)))
		case "error":
			b.WriteString(bootErrorStyle.Render("\n  Boot sequence encountered errors"))
		}
		b.WriteString("\n")
	}

	// Footer with countdown
	var footerText string
	if m.phase == "countdown" && m.countdown > 0 {
		// Countdown timer display
		countdownStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffdab3ff"))

		footerText = fmt.Sprintf("\n  %s Starting server in %s seconds...\n  Press 'q' to skip and continue now",
			bootFrames[m.animFrame%len(bootFrames)],
			countdownStyle.Render(fmt.Sprintf("%d", m.countdown)),
			// progressBar,
		)
	} else {
		footerText = "Press 'q' to continue..."
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56575eff")).
		Render(footerText)
	b.WriteString("\n")
	b.WriteString(footer)

	// Wrap entire content with padding
	containerStyle := lipgloss.NewStyle().Padding(2)
	return containerStyle.Render(b.String())
}

func (m BootModel) renderBootServices() string {
	var lines []string

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f0ca8c")).
		Render("◆ Boot Sequence")
	lines = append(lines, header)
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#44475A")).Render(strings.Repeat("─", 100)))

	for i, r := range m.results {
		var icon, status string
		var statusStyle lipgloss.Style

		switch r.Status {
		case "pending":
			icon = bootPendingIcon
			status = "waiting"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
		case "loading":
			icon = m.spinner.View()
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f0ca8c"))
		case "success":
			icon = bootSuccessIcon
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#95ffafff"))
		case "error":
			icon = bootErrorIcon
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
		case "skipped":
			icon = bootSkipIcon
			status = "disabled"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#44475A")).Italic(true)
		}

		nameStyle := lipgloss.NewStyle().Width(60)
		if i == m.current-1 && r.Status == "loading" {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#FFB86C")).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color("#F8F8F2"))
		}

		line := fmt.Sprintf("  %s %s → %s",
			icon,
			nameStyle.Render(r.Name),
			statusStyle.Render(status),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// GetResults returns the final results after boot completes
func (m BootModel) GetResults() []ServiceStatus {
	return m.results
}

// HasErrors returns true if any service failed to initialize
func (m BootModel) HasErrors() bool {
	for _, r := range m.results {
		if r.Status == "error" {
			return true
		}
	}
	return false
}

// RunBootSequence runs the boot sequence TUI
func RunBootSequence(cfg StartupConfig, initQueue []ServiceInit) ([]ServiceStatus, error) {
	m := NewBootModel(cfg, initQueue)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if finalBoot, ok := finalModel.(BootModel); ok {
		return finalBoot.GetResults(), nil
	}

	return nil, fmt.Errorf("unexpected model type")
}
