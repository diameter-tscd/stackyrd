package tui

import (
	"fmt"
	"os"
	"stackyrd/pkg/tui/template"
	"stackyrd/pkg/utils"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LiveConfig contains configuration for the live TUI
type LiveConfig struct {
	AppName    string
	AppVersion string
	Banner     string
	Port       string
	Env        string
	OnShutdown func() // Callback function to trigger shutdown
}

// LogEntry represents a log entry
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

// LiveModel is the Bubble Tea model for the live running dashboard
type LiveModel struct {
	spinner         spinner.Model
	textinput       textinput.Model
	config          LiveConfig
	allLogs         []LogEntry
	filteredLogs    []LogEntry
	logsMutex       sync.RWMutex
	filterText      string
	scrollOffset    int  // Current scroll position in the log list
	maxVisibleLines int  // Maximum number of log lines to show
	autoScroll      bool // Whether to auto-scroll to bottom on new logs
	startTime       time.Time
	width           int
	height          int
	frame           int
	quitting        bool
	maxLogs         int
	program         *tea.Program

	// Reusable dialog components
	exitDialog   *template.DialogModel
	filterDialog *template.DialogModel
	queryDialog  *template.DialogModel
}

// Live TUI styles
var (
	liveBannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8daea5"))

	liveTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffffff"))

	liveInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8daea5"))

	liveStatusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8daea5"))

	liveDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262ff"))

	liveLogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8daea5")).
			Padding(0, 1)

	// Single cyan color for progress bar
	liveProgressColor = "#8daea5"
)

// NewLiveModel creates a new live TUI model
func NewLiveModel(cfg LiveConfig) *LiveModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))

	// Initialize text input for filtering
	ti := textinput.New()
	ti.Placeholder = "Filter logs..."
	ti.CharLimit = 50
	ti.Width = 30
	// Make sure the text input is visible with a border
	ti.Prompt = ""
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))

	// Initialize reusable dialogs
	exitDialog := template.NewExitConfirmationDialog()
	filterDialog := template.NewFilterDialog("")
	queryDialog := template.NewQueryDialog("")

	return &LiveModel{
		spinner:         s,
		textinput:       ti,
		config:          cfg,
		allLogs:         make([]LogEntry, 0),
		filteredLogs:    make([]LogEntry, 0),
		maxVisibleLines: 15,   // Default number of log lines to show
		autoScroll:      true, // Start with auto-scroll enabled
		startTime:       time.Now(),
		width:           80,
		height:          24,
		maxLogs:         1000, // Unlimited logs (0 disables the limit)
		exitDialog:      exitDialog,
		filterDialog:    filterDialog,
		queryDialog:     queryDialog,
	}
}

type liveTickMsg time.Time
type logMsg LogEntry

func liveTickCmd() tea.Cmd {
	return tea.Every(time.Millisecond*100, func(t time.Time) tea.Msg {
		return liveTickMsg(t)
	})
}

func (m *LiveModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		liveTickCmd(),
	)
}

func (m *LiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle active dialogs first
		if m.exitDialog.IsActive() {
			cmd := m.exitDialog.Update(msg)
			if result := m.exitDialog.GetResult(); result != nil {
				if result.Confirmed {
					// Confirm exit
					m.quitting = true
					// Call the shutdown callback if provided
					if m.config.OnShutdown != nil {
						m.config.OnShutdown()
					}
					return m, tea.Quit
				}
				// Dialog was cancelled or dismissed
			}
			return m, cmd
		}

		if m.filterDialog.IsActive() {
			cmd := m.filterDialog.Update(msg)
			if result := m.filterDialog.GetResult(); result != nil {
				if result.Confirmed {
					// Apply filter
					m.filterText = result.Value
					m.updateFilteredLogs()
					m.scrollToTop() // Scroll to top to show first filtered results
				} else {
					// Filter cancelled, reset
					m.filterText = ""
					m.updateFilteredLogs()
				}
			}
			return m, cmd
		}

		if m.queryDialog.IsActive() {
			cmd := m.queryDialog.Update(msg)
			if result := m.queryDialog.GetResult(); result != nil {
				if result.Confirmed {
					if result.Value != "" {
						m.updateQuery(result.Value)
					}
				}
			}
			return m, cmd
		}

		// Handle normal navigation
		switch msg.String() {
		case "ctrl+c":
			// Show exit confirmation dialog
			m.exitDialog.Show()
			return m, nil
		case "/":
			// Show filter dialog
			m.filterDialog.Show()
			return m, nil
		case "ctrl+p":
			// Show query dialog
			m.queryDialog.Show()
			return m, nil
		case "down", "j":
			// Scroll down
			m.scrollDown()
			return m, nil
		case "up", "k":
			// Scroll up
			m.scrollUp()
			return m, nil
		case "pgdown", " ":
			// Page down
			m.pageDown()
			return m, nil
		case "pgup":
			// Page up
			m.pageUp()
			return m, nil
		case "home", "g":
			// Go to top
			m.scrollToTop()
			return m, nil
		case "end", "G":
			// Go to bottom
			m.scrollToBottom()
			return m, nil
		case "ctrl+l":
			// Toggle auto-scroll
			m.autoScroll = !m.autoScroll
			if m.autoScroll {
				// If enabling auto-scroll, jump to bottom
				m.scrollToBottom()
			}
			return m, nil
		case "f2":
			// Clear all logs
			m.clearLogs()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update max visible lines based on available height
		// Account for header (4 lines), status (1 line), borders/padding (4 lines), footer (1 line)
		m.maxVisibleLines = msg.Height - 10
		if m.maxVisibleLines < 5 {
			m.maxVisibleLines = 5
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// case liveTickMsg:
	// 	m.frame = (m.frame + 1) % len(loopingProgressFrames)
	// 	return m, tea.Batch(m.spinner.Tick, liveTickCmd())

	case logMsg:
		m.logsMutex.Lock()
		m.allLogs = append(m.allLogs, LogEntry(msg))
		// Keep only the last maxLogs entries (if maxLogs > 0)
		if m.maxLogs > 0 && len(m.allLogs) > m.maxLogs {
			m.allLogs = m.allLogs[len(m.allLogs)-m.maxLogs:]
		}
		m.updateFilteredLogs()

		// Auto-scroll to bottom if enabled
		if m.autoScroll {
			logsToShow := m.filteredLogs
			if m.filterText == "" {
				logsToShow = m.allLogs
			}

			// Calculate available height (same as in View method)
			totalHeight := m.height
			if totalHeight == 0 {
				totalHeight = 24 // default fallback
			}

			headerHeight := 7 // banner(1) + title(1) + status(1) + spacing(1) + logs header(1) + border(1) + spacing(1)
			if m.config.Banner != "" {
				headerHeight++ // extra line for banner
			}
			if m.filterDialog.IsActive() {
				headerHeight += 3 // filter input (1) + spacing (2)
			}

			footerHeight := 2                                                // footer + spacing
			availableHeight := totalHeight - headerHeight - footerHeight - 2 // reduced padding
			if availableHeight < 3 {
				availableHeight = 3
			}

			// Auto-scroll to bottom
			m.scrollOffset = len(logsToShow) - availableHeight
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		}

		m.logsMutex.Unlock()
		return m, nil
	}

	return m, cmd
}

func (m *LiveModel) View() string {
	if m.quitting {
		return ""
	}

	// Calculate layout dimensions
	totalHeight := m.height
	if totalHeight == 0 {
		totalHeight = 24 // default fallback
	}

	// Calculate fixed header height (banner + title + status + logs header + border + spacing)
	headerHeight := 8 // banner(1) + title(1) + status(1) + spacing(1) + logs header(1) + border(1) + spacing(1)
	if m.config.Banner != "" {
		headerHeight++ // extra line for banner
	}

	// Fixed footer height
	footerHeight := 2 // footer + spacing

	// Available height for log entries only (subtract padding)
	availableHeight := totalHeight - headerHeight - footerHeight - 2 // reduced padding
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Update max visible lines based on calculated available space
	m.maxVisibleLines = availableHeight

	// If auto-scroll is enabled, ensure we're at the bottom
	if m.autoScroll {
		logsToShow := m.filteredLogs
		if m.filterText == "" {
			logsToShow = m.allLogs
		}
		m.scrollOffset = len(logsToShow) - availableHeight
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
	}

	var b strings.Builder

	// STICKY HEADER - Always visible at the top

	// Main content
	var mainContent strings.Builder

	// STICKY HEADER - Always visible at the top

	// Sticky Banner
	if m.config.Banner != "" {
		mainContent.WriteString(liveBannerStyle.Render(m.config.Banner))
		mainContent.WriteString("\n")
	}

	// Header with app info (cyan accent)
	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(liveProgressColor)).Bold(true)

	header := fmt.Sprintf("%s %s v%s %s",
		cyanStyle.Render(" "),
		liveTitleStyle.Render(m.config.AppName),
		m.config.AppVersion,
		cyanStyle.Render(" "),
	)
	mainContent.WriteString(header)
	mainContent.WriteString("\n")

	// Status line
	uptime := time.Since(m.startTime).Round(time.Second)
	statusLine := fmt.Sprintf("  %s %s ● Service Port: %s ● Env: %s ● Usage: %s ● Routine: %s ● Uptime: %s",
		m.spinner.View(),
		liveStatusStyle.Render("RUNNING"),
		liveInfoStyle.Render(m.config.Port),
		liveInfoStyle.Render(m.config.Env),
		liveInfoStyle.Render(fmt.Sprintf("%d MiB", utils.GetMemSelf())),
		liveInfoStyle.Render(fmt.Sprintf("%d", utils.GetRoutine())),
		liveInfoStyle.Render(uptime.String()),
	)
	mainContent.WriteString(statusLine)
	mainContent.WriteString("\n\n")

	// STICKY LOGS HEADER - Always visible
	logWidth := m.width - 4 // account for container padding
	if logWidth < 56 {
		logWidth = 56
	}
	if logWidth > 136 {
		logWidth = 136
	}

	stickyLogsHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#626262ff")).
		Render("▪ Live Logs")
	mainContent.WriteString(stickyLogsHeader)
	mainContent.WriteString("\n")
	mainContent.WriteString(liveDimStyle.Render(strings.Repeat("─", logWidth)))
	mainContent.WriteString("\n")

	// SCROLLABLE CONTENT - Only the log entries (no header/border)
	logLines := m.renderLogEntriesOnly()
	if len(logLines) > availableHeight {
		// Apply scrolling offset to log entries only
		startLine := m.scrollOffset
		if startLine >= len(logLines) {
			startLine = len(logLines) - 1
		}
		if startLine < 0 {
			startLine = 0
		}

		endLine := startLine + availableHeight
		if endLine > len(logLines) {
			endLine = len(logLines)
		}

		logLines = logLines[startLine:endLine]
	}

	// Render visible log entries
	for _, line := range logLines {
		mainContent.WriteString(line)
		mainContent.WriteString("\n")
	}

	// Fill remaining space to push footer to bottom
	remainingLines := availableHeight - len(logLines)
	if remainingLines > 0 {
		for i := 0; i < remainingLines; i++ {
			mainContent.WriteString("\n")
		}
	}

	// STICKY FOOTER - Always visible at the bottom
	var footerText string
	if m.filterDialog.IsActive() {
		footerText = liveDimStyle.Render("Enter: apply filter ● Esc: cancel")
	} else if m.queryDialog.IsActive() {
		footerText = liveDimStyle.Render("Enter: exec query ● Esc: cancel")
	} else {
		filterInfo := ""
		if m.filterText != "" {
			filterInfo = fmt.Sprintf("Filter: '%s' ● ", m.filterText)
		}
		autoScrollInfo := ""
		if m.autoScroll {
			autoScrollInfo = "Auto-scroll: ON ● "
		}
		footerText = liveDimStyle.Render(fmt.Sprintf("%s%sLast update: %s ● ctrl+c: exit ● /: filter ● ctrl+l: auto-scroll ● F2: clear logs",
			filterInfo, autoScrollInfo, time.Now().Format("15:04:05")))
	}
	mainContent.WriteString("\n")
	mainContent.WriteString(footerText)

	// Render main content
	b.WriteString(mainContent.String())

	// Render dialogs using reusable components
	if m.exitDialog.IsActive() {
		return m.exitDialog.View(m.width, m.height)
	}

	if m.filterDialog.IsActive() {
		return m.filterDialog.View(m.width, m.height)
	}

	if m.queryDialog.IsActive() {
		return m.queryDialog.View(m.width, m.height)
	}

	// Wrap entire content with minimal padding
	containerStyle := lipgloss.NewStyle().Padding(1)
	return containerStyle.Render(b.String())
}

// renderLogEntriesOnly returns only the log entry lines as a slice (no header/border)
func (m *LiveModel) renderLogEntriesOnly() []string {
	var lines []string

	// Calculate available width for logs content
	logWidth := m.width - 4 // account for container padding
	if logWidth < 56 {
		logWidth = 56
	}
	if logWidth > 136 {
		logWidth = 136
	}

	m.logsMutex.RLock()
	defer m.logsMutex.RUnlock()

	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}

	if len(logsToShow) == 0 {
		lines = append(lines, liveDimStyle.Render("  Waiting for logs..."))
	} else {
		for _, log := range logsToShow {
			levelStyle := m.getLevelStyle(log.Level)
			timeStr := log.Time.Format("15:04:05")
			levelStr := fmt.Sprintf("[%-5s]", strings.ToUpper(log.Level))

			// Calculate max message length and truncate before styling
			maxMsgLen := logWidth - 20 // Account for timestamp (8), level (7), spaces and prefix
			if maxMsgLen < 20 {
				maxMsgLen = 20
			}
			msg := log.Message
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-3] + "..."
			}

			// Build the line with proper formatting
			line := fmt.Sprintf("  %s %s %s",
				liveDimStyle.Render(timeStr),
				levelStyle.Render(levelStr),
				lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")).Render(msg),
			)
			lines = append(lines, line)
		}
	}

	return lines
}

func (m *LiveModel) getLevelStyle(level string) lipgloss.Style {
	switch strings.ToLower(level) {
	case "debug":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#b3ebf8ff"))
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9af8b1ff"))
	case "warn", "warning":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#f5fac0ff"))
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#f67373ff"))
	case "fatal":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#f82626ff")).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	}
}

// AddLog adds a log entry to the TUI
func (m *LiveModel) AddLog(level, message string) {
	if m.program != nil {
		m.program.Send(logMsg{
			Time:    time.Now(),
			Level:   level,
			Message: message,
		})
	}
}

// SetProgram sets the tea.Program reference for sending messages
func (m *LiveModel) SetProgram(p *tea.Program) {
	m.program = p
}

// LiveTUI manages the live TUI instance
type LiveTUI struct {
	model   *LiveModel
	program *tea.Program
}

// NewLiveTUI creates a new live TUI instance
func NewLiveTUI(cfg LiveConfig) *LiveTUI {
	model := NewLiveModel(cfg)
	return &LiveTUI{
		model: model,
	}
}

// Start starts the live TUI in a goroutine
func (t *LiveTUI) Start() {
	t.program = tea.NewProgram(t.model, tea.WithAltScreen())
	t.model.SetProgram(t.program)
	go func() {
		t.program.Run()
	}()
}

// Stop stops the live TUI
func (t *LiveTUI) Stop() {
	if t.program != nil {
		utils.ClearScreen()
		t.program.Quit()
		os.Exit(0)
	}
}

// AddLog adds a log to the live TUI
func (t *LiveTUI) AddLog(level, message string) {
	t.model.AddLog(level, message)
}

// Write implements io.Writer for use as a log broadcaster
func (t *LiveTUI) Write(p []byte) (n int, err error) {
	// Parse the log line and add it
	line := strings.TrimSpace(string(p))
	if line != "" {
		level, message := parseLogLine(line)
		if message != "" {
			t.AddLog(level, message)
		}
	}
	return len(p), nil
}

// parseLogLine extracts the level and clean message from a zerolog console output line
// Example input: "15:00:51 INF Scheduled Cron Job job=health_check schedule="*/10 * * * * *""
// Returns: level="info", message="Scheduled Cron Job job=health_check schedule="*/10 * * * * *""
func parseLogLine(line string) (level, message string) {
	level = "info" // default

	// Split by space to find components
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return level, line
	}

	// Check if first part is a timestamp (HH:MM:SS format)
	if len(parts[0]) == 8 && strings.Count(parts[0], ":") == 2 {
		// Second part should be the level abbreviation
		switch strings.ToUpper(parts[1]) {
		case "DBG", "DEBUG":
			level = "debug"
		case "INF", "INFO":
			level = "info"
		case "WRN", "WARN", "WARNING":
			level = "warn"
		case "ERR", "ERROR":
			level = "error"
		case "FTL", "FATAL":
			level = "fatal"
		case "PNC", "PANIC":
			level = "fatal"
		}

		// Message is everything after timestamp and level
		if len(parts) >= 3 {
			message = parts[2]
		} else {
			message = ""
		}
	} else {
		// No timestamp, try to detect level from content
		upperLine := strings.ToUpper(line)
		switch {
		case strings.Contains(upperLine, "DEBUG") || strings.Contains(upperLine, "DBG"):
			level = "debug"
		case strings.Contains(upperLine, "WARN") || strings.Contains(upperLine, "WRN"):
			level = "warn"
		case strings.Contains(upperLine, "ERROR") || strings.Contains(upperLine, "ERR"):
			level = "error"
		case strings.Contains(upperLine, "FATAL") || strings.Contains(upperLine, "FTL"):
			level = "fatal"
		}
		message = line
	}

	return level, message
}

// updateFilteredLogs filters the logs based on filterText
func (m *LiveModel) updateFilteredLogs() {
	if m.filterText == "" {
		// No filter, show all logs
		m.filteredLogs = make([]LogEntry, len(m.allLogs))
		copy(m.filteredLogs, m.allLogs)
		return
	}

	filterLower := strings.ToLower(m.filterText)
	var filtered []LogEntry

	for _, log := range m.allLogs {
		if strings.Contains(strings.ToLower(log.Level), filterLower) ||
			strings.Contains(strings.ToLower(log.Message), filterLower) {
			filtered = append(filtered, log)
		}
	}

	m.filteredLogs = filtered
}

func (m *LiveModel) updateQuery(query string) {
	// execute query
	go func() {
		m.AddLog("info", "Execute query: "+query)
	}()
}

// Scroll methods for navigating through logs
func (m *LiveModel) scrollDown() {
	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}

	if m.scrollOffset < len(logsToShow)-m.maxVisibleLines {
		m.scrollOffset++
		m.autoScroll = false // Disable auto-scroll when user manually scrolls
	}
}

func (m *LiveModel) scrollUp() {
	if m.scrollOffset > 0 {
		m.scrollOffset--
		m.autoScroll = false // Disable auto-scroll when user manually scrolls
	}
}

func (m *LiveModel) pageDown() {
	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}

	m.scrollOffset += m.maxVisibleLines
	maxOffset := len(logsToShow) - m.maxVisibleLines
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = false // Disable auto-scroll when user manually scrolls
}

func (m *LiveModel) pageUp() {
	m.scrollOffset -= m.maxVisibleLines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = false // Disable auto-scroll when user manually scrolls
}

func (m *LiveModel) scrollToTop() {
	m.scrollOffset = 0
	m.autoScroll = false // Disable auto-scroll when user manually scrolls
}

func (m *LiveModel) scrollToBottom() {
	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}

	m.scrollOffset = len(logsToShow) - m.maxVisibleLines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = true // Re-enable auto-scroll when user scrolls to bottom
}

// clearLogs clears all log entries and resets the view state
func (m *LiveModel) clearLogs() {
	m.logsMutex.Lock()
	defer m.logsMutex.Unlock()

	// Clear all logs
	m.allLogs = make([]LogEntry, 0)
	m.filteredLogs = make([]LogEntry, 0)

	// Reset scroll and filter state
	m.scrollOffset = 0
	m.filterText = ""
	m.textinput.SetValue("")

	// Keep auto-scroll state as-is
}

// RunLiveTUI runs the live TUI and blocks until quit
func RunLiveTUI(cfg LiveConfig) error {
	model := NewLiveModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)
	_, err := p.Run()
	return err
}
