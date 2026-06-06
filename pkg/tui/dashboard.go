package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// DashboardConfig contains configuration for the dashboard TUI
type DashboardConfig struct {
	AppName    string
	AppVersion string
	Port       string
	Env        string
	StartTime  time.Time
}

// InfraStatus represents infrastructure component status
type InfraStatus struct {
	Name      string
	Enabled   bool
	Connected bool
}

// DashboardModel is the Bubble Tea model for the live dashboard
type DashboardModel struct {
	spinner       spinner.Model
	viewport      viewport.Model
	textinput     textinput.Model
	config        DashboardConfig
	allInfra      []InfraStatus
	allServices   []ServiceStatus
	filteredInfra []InfraStatus
	filteredSvc   []ServiceStatus
	filterText    string
	showFilter    bool
	cpuPercent    float64
	memPercent    float64
	memUsed       uint64
	memTotal      uint64
	goroutines    int
	lastUpdate    time.Time
	width         int
	height        int
	frame         int // For animation frames
	quitting      bool
}

// Dashboard styles
var (
	dashTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8daea5")).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 2)

	dashBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Padding(0, 1)

	dashHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8BE9FD")).
			MarginBottom(1)

	dashLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	dashValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Bold(true)

	dashGoodStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B"))

	dashWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1FA8C"))

	dashBadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555"))

	dashDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44475A"))

	dashAccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9"))

	dashPulseColors = []string{"#FF79C6", "#BD93F9", "#8BE9FD", "#50FA7B", "#F1FA8C", "#FFB86C", "#FF5555"}
)

// Animation frames for the running indicator
var runningFrames = []string{
	"▰▱▱▱▱▱▱",
	"▰▰▱▱▱▱▱",
	"▰▰▰▱▱▱▱",
	"▰▰▰▰▱▱▱",
	"▰▰▰▰▰▱▱",
	"▰▰▰▰▰▰▱",
	"▰▰▰▰▰▰▰",
	"▱▰▰▰▰▰▰",
	"▱▱▰▰▰▰▰",
	"▱▱▱▰▰▰▰",
	"▱▱▱▱▰▰▰",
	"▱▱▱▱▱▰▰",
	"▱▱▱▱▱▱▰",
	"▱▱▱▱▱▱▱",
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(cfg DashboardConfig, infra []InfraStatus, services []ServiceStatus) DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	// Initialize viewport
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Initialize text input for filtering
	ti := textinput.New()
	ti.Placeholder = "Filter services/infra..."
	ti.CharLimit = 50
	ti.Width = 30

	return DashboardModel{
		spinner:       s,
		viewport:      vp,
		textinput:     ti,
		config:        cfg,
		allInfra:      infra,
		allServices:   services,
		filteredInfra: infra,
		filteredSvc:   services,
		lastUpdate:    time.Now(),
		width:         80,
		height:        24,
	}
}

type dashTickMsg time.Time

func dashTickCmd() tea.Cmd {
	return tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg {
		return dashTickMsg(t)
	})
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		dashTickCmd(),
	)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle text input when filter is active
		if m.showFilter {
			switch msg.String() {
			case "enter":
				m.showFilter = false
				m.filterText = m.textinput.Value()
				m.updateFilteredLists()
				return m, nil
			case "esc":
				m.showFilter = false
				m.textinput.SetValue("")
				m.filterText = ""
				m.updateFilteredLists()
				return m, nil
			default:
				var tiCmd tea.Cmd
				m.textinput, tiCmd = m.textinput.Update(msg)
				return m, tiCmd
			}
		}

		// Handle normal navigation
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			// Enable filter mode
			m.showFilter = true
			m.textinput.Focus()
			return m, nil
		case "down", "j":
			// Scroll down
			m.viewport.ScrollDown(1)
			return m, nil
		case "up", "k":
			// Scroll up
			m.viewport.ScrollUp(1)
			return m, nil
		case "pgdown", " ":
			// Page down
			m.viewport.HalfPageDown()
			return m, nil
		case "pgup":
			// Page up
			m.viewport.HalfPageUp()
			return m, nil
		case "home", "g":
			// Go to top
			m.viewport.GotoTop()
			return m, nil
		case "end", "G":
			// Go to bottom
			m.viewport.GotoBottom()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update viewport size (leave room for header and footer)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 8

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dashTickMsg:
		m.frame = (m.frame + 1) % len(runningFrames)
		m.lastUpdate = time.Now()
		m.goroutines = runtime.NumGoroutine()

		// Update system stats
		if v, err := mem.VirtualMemory(); err == nil {
			m.memPercent = v.UsedPercent
			m.memUsed = v.Used / 1024 / 1024
			m.memTotal = v.Total / 1024 / 1024
		}
		if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
			m.cpuPercent = c[0]
		}

		return m, tea.Batch(m.spinner.Tick, dashTickCmd())
	}

	return m, cmd
}

func (m DashboardModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Filter input at the top when active
	if m.showFilter {
		filterPrompt := dashAccentStyle.Render("Filter: ")
		b.WriteString(filterPrompt)
		b.WriteString(m.textinput.View())
		b.WriteString("\n\n")
	}

	// Header with pulsing color
	pulseColor := dashPulseColors[m.frame%len(dashPulseColors)]
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(pulseColor))

	header := "╭─────────────────────────────────────────────╮"
	b.WriteString(dashDimStyle.Render(header))
	b.WriteString("\n")

	title := fmt.Sprintf("│  %s  %s v%s  %s  │",
		headerStyle.Render("⚡"),
		dashTitleStyle.Render(m.config.AppName),
		m.config.AppVersion,
		headerStyle.Render("⚡"),
	)
	b.WriteString(title)
	b.WriteString("\n")

	footerLine := "╰─────────────────────────────────────────────╯"
	b.WriteString(dashDimStyle.Render(footerLine))
	b.WriteString("\n\n")

	// Running animation
	animation := lipgloss.NewStyle().Foreground(lipgloss.Color(pulseColor)).Render(runningFrames[m.frame])
	uptime := time.Since(m.config.StartTime).Round(time.Second)
	statusLine := fmt.Sprintf("  %s %s  Uptime: %s  Port: %s  Env: %s",
		m.spinner.View(),
		animation,
		dashValueStyle.Render(uptime.String()),
		dashAccentStyle.Render(m.config.Port),
		dashValueStyle.Render(m.config.Env),
	)
	b.WriteString(statusLine)
	b.WriteString("\n\n")

	// Create content for viewport
	var content strings.Builder

	// System Resources Box
	systemBox := m.renderSystemBox()
	content.WriteString(systemBox)
	content.WriteString("\n\n")

	// Infrastructure Box
	infraBox := m.renderInfraBox()
	content.WriteString(infraBox)
	content.WriteString("\n\n")

	// Services Box
	servicesBox := m.renderServicesBox()
	content.WriteString(servicesBox)
	content.WriteString("\n")

	// Set content in viewport
	m.viewport.SetContent(content.String())

	// Render viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Footer with controls
	var footerText string
	if m.showFilter {
		footerText = dashDimStyle.Render("Enter: apply filter │ Esc: cancel")
	} else {
		filterInfo := ""
		if m.filterText != "" {
			filterInfo = fmt.Sprintf("Filter: '%s' │ ", m.filterText)
		}
		scrollInfo := fmt.Sprintf("Scroll: %d/%d", m.viewport.YOffset+1, len(strings.Split(content.String(), "\n")))
		footerText = dashDimStyle.Render(fmt.Sprintf("%sLast update: %s │ %s │ q: exit │ /: filter │ ↑↓: scroll",
			filterInfo, m.lastUpdate.Format("15:04:05"), scrollInfo))
	}
	b.WriteString(footerText)

	return b.String()
}

func (m DashboardModel) renderSystemBox() string {
	var lines []string
	lines = append(lines, dashHeaderStyle.Render("⊙ System Resources"))

	// CPU with color-coded bar
	cpuBar := m.renderProgressBar(m.cpuPercent, 15)
	cpuLine := fmt.Sprintf("%s %s %s",
		dashLabelStyle.Render("CPU:"),
		cpuBar,
		m.getPercentStyle(m.cpuPercent).Render(fmt.Sprintf("%.1f%%", m.cpuPercent)),
	)
	lines = append(lines, cpuLine)

	// Memory with color-coded bar
	memBar := m.renderProgressBar(m.memPercent, 15)
	memLine := fmt.Sprintf("%s %s %s",
		dashLabelStyle.Render("RAM:"),
		memBar,
		m.getPercentStyle(m.memPercent).Render(fmt.Sprintf("%.1f%%", m.memPercent)),
	)
	lines = append(lines, memLine)

	memDetail := fmt.Sprintf("     %s / %s MB",
		dashValueStyle.Render(fmt.Sprintf("%d", m.memUsed)),
		dashDimStyle.Render(fmt.Sprintf("%d", m.memTotal)),
	)
	lines = append(lines, memDetail)

	// Goroutines
	goLine := fmt.Sprintf("%s %s",
		dashLabelStyle.Render("Goroutines:"),
		dashValueStyle.Render(fmt.Sprintf("%d", m.goroutines)),
	)
	lines = append(lines, goLine)

	content := strings.Join(lines, "\n")
	return dashBoxStyle.Width(35).Render(content)
}

func (m DashboardModel) renderInfraBox() string {
	var lines []string
	lines = append(lines, dashHeaderStyle.Render("⊙ Infrastructure"))

	for _, infra := range m.filteredInfra {
		var icon string
		var style lipgloss.Style

		if !infra.Enabled {
			icon = "○"
			style = dashDimStyle
		} else if infra.Connected {
			icon = "●"
			style = dashGoodStyle
		} else {
			icon = "●"
			style = dashBadStyle
		}

		status := "disabled"
		if infra.Enabled {
			if infra.Connected {
				status = "connected"
			} else {
				status = "disconnected"
			}
		}

		line := fmt.Sprintf("%s %s %s",
			style.Render(icon),
			dashLabelStyle.Width(12).Render(infra.Name+":"),
			style.Render(status),
		)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return dashBoxStyle.Width(30).Render(content)
}

func (m DashboardModel) renderServicesBox() string {
	var lines []string
	lines = append(lines, dashHeaderStyle.Render("⊙ Services"))

	for _, svc := range m.filteredSvc {
		var icon string
		var style lipgloss.Style

		switch svc.Status {
		case "success":
			icon = "●"
			style = dashGoodStyle
		case "loading":
			icon = m.spinner.View()
			style = dashWarnStyle
		case "error":
			icon = "●"
			style = dashBadStyle
		case "skipped":
			icon = "○"
			style = dashDimStyle
		default:
			icon = "○"
			style = dashDimStyle
		}

		statusText := svc.Status
		if svc.Status == "success" {
			statusText = "running"
		}

		line := fmt.Sprintf("  %s %s %s",
			style.Render(icon),
			dashLabelStyle.Width(15).Render(svc.Name+":"),
			style.Render(statusText),
		)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return dashBoxStyle.Render(content)
}

func (m DashboardModel) renderProgressBar(percent float64, width int) string {
	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStyle := m.getPercentStyle(percent)
	bar := filledStyle.Render(strings.Repeat("█", filled)) + dashDimStyle.Render(strings.Repeat("░", empty))
	return bar
}

func (m DashboardModel) getPercentStyle(percent float64) lipgloss.Style {
	switch {
	case percent < 50:
		return dashGoodStyle
	case percent < 80:
		return dashWarnStyle
	default:
		return dashBadStyle
	}
}

// updateFilteredLists filters the infrastructure and services based on filterText
func (m *DashboardModel) updateFilteredLists() {
	if m.filterText == "" {
		// No filter, show all
		m.filteredInfra = m.allInfra
		m.filteredSvc = m.allServices
		return
	}

	filterLower := strings.ToLower(m.filterText)

	// Filter infrastructure
	var filteredInfra []InfraStatus
	for _, infra := range m.allInfra {
		if strings.Contains(strings.ToLower(infra.Name), filterLower) {
			filteredInfra = append(filteredInfra, infra)
		}
	}
	m.filteredInfra = filteredInfra

	// Filter services
	var filteredSvc []ServiceStatus
	for _, svc := range m.allServices {
		if strings.Contains(strings.ToLower(svc.Name), filterLower) ||
			strings.Contains(strings.ToLower(svc.Status), filterLower) ||
			strings.Contains(strings.ToLower(svc.Message), filterLower) {
			filteredSvc = append(filteredSvc, svc)
		}
	}
	m.filteredSvc = filteredSvc
}

// RunDashboardTUI runs the dashboard TUI
func RunDashboardTUI(cfg DashboardConfig, infra []InfraStatus, services []ServiceStatus) error {
	m := NewDashboardModel(cfg, infra, services)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
