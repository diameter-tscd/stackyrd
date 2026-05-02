package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServiceStatus represents the status of a service during startup
type ServiceStatus struct {
	Name    string
	Status  string // "pending", "loading", "success", "error", "skipped"
	Message string
}

// StartupConfig contains configuration for the startup TUI
type StartupConfig struct {
	AppName     string
	AppVersion  string
	Banner      string
	Port        string
	Env         string
	IdleSeconds int // How long to display the boot screen (0 to skip immediately)
}

// StartupModel is the Bubble Tea model for startup animation
type StartupModel struct {
	spinner   spinner.Model
	progress  progress.Model
	services  []ServiceStatus
	current   int
	done      bool
	config    StartupConfig
	startTime time.Time
	width     int
}

// Styles
var (
	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8daea5")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Italic(true)

	// Banner style with gradient effect
	bannerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9")).
			Bold(true).
			MarginBottom(1)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(1, 2).
			MarginTop(1)

	// Service status styles
	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555"))

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)

	// Info styles
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Bold(true)

	// Footer style
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			MarginTop(1)

	// Highlight style
	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Bold(true)
)

// Icons
const (
	iconPending = "○"
	iconLoading = "◐"
	iconSuccess = "✓"
	iconError   = "✗"
	iconSkipped = "⊘"
	iconSparkle = "✨"
	iconServer  = "⚡"
	iconCheck   = "✔"
	iconArrow   = "→"
)

// Messages
type tickMsg time.Time
type doneMsg struct{}

// NewStartupModel creates a new startup TUI model
func NewStartupModel(cfg StartupConfig, services []ServiceStatus) StartupModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return StartupModel{
		spinner:   s,
		progress:  p,
		services:  services,
		config:    cfg,
		startTime: time.Now(),
		width:     80,
	}
}

func (m StartupModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Every(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m StartupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.progress.Width = min(msg.Width-20, 60)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Simulate service initialization progress
		if m.current < len(m.services) {
			// Update current service to loading
			switch m.services[m.current].Status {
			case "pending":
				m.services[m.current].Status = "loading"
				m.services[m.current].Message = "Initializing..."
			case "loading":
				// Complete current service and move to next
				m.services[m.current].Status = "success"
				m.services[m.current].Message = "Ready"
				m.current++
			}
			return m, tea.Batch(m.spinner.Tick, tickCmd())
		} else if !m.done {
			m.done = true
			// Brief delay before exiting
			return m, tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
				return doneMsg{}
			})
		}

	case doneMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m StartupModel) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// ASCII Art Banner with gradient
	if m.config.Banner != "" {
		banner := bannerStyle.Render(m.config.Banner)
		b.WriteString(banner)
		b.WriteString("\n")
	}

	// Title
	title := fmt.Sprintf("%s %s", iconSparkle, m.config.AppName)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	subtitle := fmt.Sprintf("Version %s • Environment: %s", m.config.AppVersion, m.config.Env)
	b.WriteString(subtitleStyle.Render(subtitle))
	b.WriteString("\n\n")

	// Progress bar
	completed := 0
	for _, s := range m.services {
		if s.Status == "success" || s.Status == "skipped" || s.Status == "error" {
			completed++
		}
	}
	progressPercent := float64(completed) / float64(len(m.services))
	b.WriteString(m.progress.ViewAs(progressPercent))
	b.WriteString(fmt.Sprintf(" %d/%d\n\n", completed, len(m.services)))

	// Services list
	servicesContent := m.renderServices()
	b.WriteString(boxStyle.Render(servicesContent))
	b.WriteString("\n")

	// Server info when done
	if m.done {
		elapsed := time.Since(m.startTime).Round(time.Millisecond)
		serverInfo := fmt.Sprintf("\n%s Server running at %s:%s\n",
			iconServer,
			highlightStyle.Render("http://localhost"),
			highlightStyle.Render(m.config.Port),
		)
		b.WriteString(successStyle.Render(serverInfo))

		readyMsg := fmt.Sprintf("%s Ready in \n", elapsed)
		b.WriteString(successStyle.Render(readyMsg))
	}

	// Footer
	footer := "Press 'q' to continue..."
	b.WriteString(footerStyle.Render(footer))

	// Wrap entire content with padding
	containerStyle := lipgloss.NewStyle().Padding(2)
	return containerStyle.Render(b.String())
}

func (m StartupModel) renderServices() string {
	var lines []string

	header := labelStyle.Render("◆ Services Initialization")
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("─", 40))

	for i, s := range m.services {
		var icon, status string
		var style lipgloss.Style

		switch s.Status {
		case "pending":
			icon = iconPending
			status = "Pending"
			style = pendingStyle
		case "loading":
			icon = m.spinner.View()
			status = s.Message
			style = loadingStyle
		case "success":
			icon = iconSuccess
			status = s.Message
			style = successStyle
		case "error":
			icon = iconError
			status = s.Message
			style = errorStyle
		case "skipped":
			icon = iconSkipped
			status = "Skipped"
			style = skippedStyle
		}

		// Highlight current service
		name := s.Name
		if i == m.current && s.Status == "loading" {
			name = highlightStyle.Render(name)
		}

		line := fmt.Sprintf("  %s %s %s %s",
			icon,
			lipgloss.NewStyle().Width(20).Render(name),
			iconArrow,
			style.Render(status),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// RunStartupTUI runs the startup TUI and returns when complete
func RunStartupTUI(cfg StartupConfig, services []ServiceStatus) error {
	m := NewStartupModel(cfg, services)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
