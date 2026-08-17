package tui

import (
	"fmt"
	"slices"
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
	isDone        bool
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

// Boot style functions - return fresh styles each call for theme-aware colors
func bootBannerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(TC("primary")))
}

func bootSubStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("dim"))).
		Italic(true)
}

func bootCompleteStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(TC("dim")))
}

func bootErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(TC("error")))
}

func bootPhaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("warning"))).
		Bold(true)
}

func bootInfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("secondary")))
}

func bootSuccessIcon() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("success"))).
		Render("✓")
}

func bootErrorIcon() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("error"))).
		Render("✗")
}

func bootSkipIcon() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("dim"))).
		Render("○")
}

func bootPendingIcon() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("dim"))).
		Render("◦")
}

// Messages for boot model
type bootTickMsg time.Time
type bootDoneMsg struct{}

// bootInitResultMsg carries a service init outcome back to the event loop so a
// slow InitFunc never blocks the Bubble Tea Update goroutine.
type bootInitResultMsg struct {
	index int
	err   error
}

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
				m.isDone = true
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

				// Run initialization off the event loop; the result arrives as
				// bootInitResultMsg so a slow init cannot freeze the spinner.
				svc := m.initQueue[m.current]
				idx := m.current
				if svc.InitFunc != nil {
					return m, tea.Batch(m.spinner.Tick, bootTickCmd(), func() tea.Msg {
						return bootInitResultMsg{index: idx, err: svc.InitFunc()}
					})
				}
				m.results[idx].Status = "success"
				m.results[idx].Message = "Ready"
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

	case bootInitResultMsg:
		if msg.index < len(m.results) {
			if msg.err != nil {
				m.results[msg.index].Status = "error"
				m.results[msg.index].Message = msg.err.Error()
			} else {
				m.results[msg.index].Status = "success"
				m.results[msg.index].Message = "Ready"
			}
			if msg.index == m.current {
				m.current++
			}
		}
		return m, tea.Batch(m.spinner.Tick, bootTickCmd())

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

	// Banner (ASCII art) or app name fallback
	if m.config.Banner != "" {
		b.WriteString(bootBannerStyle().Render(m.config.Banner))
	} else {
		title := fmt.Sprintf(" %s ", m.config.AppName)
		b.WriteString(bootBannerStyle().Bold(true).Render(title))
	}
	b.WriteString("\n")

	// Version and env
	sub := fmt.Sprintf("v%s • %s environment", m.config.AppVersion, m.config.Env)
	b.WriteString(bootSubStyle().Render(sub))
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
	fmt.Fprintf(&b, "%s %s\n\n", phaseIcon, bootPhaseStyle().Render(phaseText))

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
		fmt.Fprintf(&b, "Progress: %d/%d services\n\n", completed, total)
	}

	// Services list
	servicesContent := m.renderBootServices()
	b.WriteString(servicesContent)
	b.WriteString("\n")

	// Final message
	if m.isDone {
		elapsed := time.Since(m.startTime).Round(time.Millisecond)

		switch m.phase {
		case "complete":
			msg := fmt.Sprintf("\n Server ready at http://localhost:%s", m.config.Port)
			b.WriteString(bootCompleteStyle().Render(msg))
			b.WriteString("\n")
			b.WriteString(bootInfoStyle().Render(fmt.Sprintf(" Started in %s", elapsed)))
		case "error":
			b.WriteString(bootErrorStyle().Render("\n  Boot sequence encountered errors"))
		}
		b.WriteString("\n")
	}

	// Footer with countdown
	var footerText string
	if m.phase == "countdown" && m.countdown > 0 {
		// Countdown timer display
		countdownStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(TC("warning")))

		footerText = fmt.Sprintf("\n  %s Starting server in %s seconds...\n  Press 'q' to skip and continue now",
			bootFrames[m.animFrame%len(bootFrames)],
			countdownStyle.Render(fmt.Sprintf("%d", m.countdown)),
			// progressBar,
		)
	} else {
		footerText = "Press 'q' to continue..."
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("dim"))).
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
		Foreground(lipgloss.Color(TC("warning"))).
		Render("◆ Boot Sequence")
	lines = append(lines, header)
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim"))).Render(strings.Repeat("─", 100)))

	for i, r := range m.results {
		var icon, status string
		var statusStyle lipgloss.Style

		switch r.Status {
		case "pending":
			icon = bootPendingIcon()
			status = "waiting"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim")))
		case "loading":
			icon = m.spinner.View()
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("warning")))
		case "success":
			icon = bootSuccessIcon()
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("success")))
		case "error":
			icon = bootErrorIcon()
			status = r.Message
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("error")))
		case "skipped":
			icon = bootSkipIcon()
			status = "disabled"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("dim"))).Italic(true)
		}

		nameStyle := lipgloss.NewStyle().Width(60)
		if i == m.current-1 && r.Status == "loading" {
			nameStyle = nameStyle.Foreground(lipgloss.Color(TC("warning"))).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color(TC("text")))
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
	return slices.Clone(m.results)
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
