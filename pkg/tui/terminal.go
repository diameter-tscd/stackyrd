package tui

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/tui/template"
	"stackyrd/pkg/utils"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type terminalMode int

const (
	modeNormal terminalMode = iota
	modeCommand
)

type terminalFocus int

const (
	focusSidebar terminalFocus = iota
	focusLogs
	focusCommand
)

type InfraEntry struct {
	Name      string
	Connected bool
}

type ServiceEntry struct {
	Name    string
	Running bool
}

type MiddlewareEntry struct {
	Name    string
	Enabled bool
}

type TerminalModel struct {
	spinner      spinner.Model
	commandInput textinput.Model
	config       LiveConfig

	mode  terminalMode
	focus terminalFocus

	allLogs         []LogEntry
	filteredLogs    []LogEntry
	logsMutex       sync.RWMutex
	filterText      string
	scrollOffset    int
	maxVisibleLines int
	autoScroll      bool
	startTime       time.Time
	width           int
	height          int
	quitting        bool
	maxLogs         int
	program         *tea.Program

	exitDialog   *template.DialogModel
	filterDialog *template.DialogModel

	cpuPercent float64
	memPercent float64
	memUsed    uint64
	memTotal   uint64
	goroutines int
	hostname   string

	infraEntries      []InfraEntry
	serviceEntries    []ServiceEntry
	middlewareEntries []MiddlewareEntry

	cpuModel  string
	pid       int
	appMem    uint64
	pluginLoaded int
	pluginTotal  int

	sidebarWidth         int
	sidebarContentWidth  int
	mainWidth            int
	logWidth             int
	sidebarHidden        bool
}

type terminalTickMsg time.Time

func terminalTickCmd() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		return terminalTickMsg(t)
	})
}

func NewTerminalModel(cfg LiveConfig) *TerminalModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))

	ti := textinput.New()
	ti.Placeholder = "Type :command... (ctrl+p to activate)"
	ti.CharLimit = 500
	ti.Width = 60
	ti.Prompt = ": "
	ti.PromptStyle = commandPromptStyle
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))

	exitDialog := template.NewExitConfirmationDialog()
	filterDialog := template.NewFilterDialog("")

	m := &TerminalModel{
		spinner:         s,
		commandInput:    ti,
		config:          cfg,
		allLogs:         make([]LogEntry, 0),
		filteredLogs:    make([]LogEntry, 0),
		maxVisibleLines: 15,
		autoScroll:      true,
		startTime:       time.Now(),
		width:           80,
		height:          24,
		maxLogs:         1000,
		exitDialog:      exitDialog,
		filterDialog:    filterDialog,
		mode:            modeNormal,
		focus:           focusLogs,
		pid:             os.Getpid(),
	}

	m.calculateWidths()

	if info, err := utils.GetNetworkInfo(); err == nil {
		m.hostname = info["hostname"]
	}

	return m
}

func (m *TerminalModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		terminalTickCmd(),
	)
}

func (m *TerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.exitDialog.IsActive() {
			dialogCmd := m.exitDialog.Update(msg)
			if result := m.exitDialog.GetResult(); result != nil {
				if result.Confirmed {
					m.quitting = true
					if m.config.OnShutdown != nil {
						m.config.OnShutdown()
					}
					return m, tea.Quit
				}
			}
			return m, dialogCmd
		}

		if m.filterDialog.IsActive() {
			dialogCmd := m.filterDialog.Update(msg)
			if result := m.filterDialog.GetResult(); result != nil {
				if result.Confirmed {
					m.filterText = result.Value
					m.updateFilteredLogs()
					m.scrollToTop()
				} else {
					m.filterText = ""
					m.updateFilteredLogs()
				}
			}
			return m, dialogCmd
		}

		if m.mode == modeCommand {
			switch msg.String() {
			case "enter":
				raw := m.commandInput.Value()
				m.commandInput.SetValue("")
				m.commandInput.Blur()
				m.mode = modeNormal
				if raw != "" {
					return m, m.executeCommand(raw)
				}
				return m, nil
			case "esc":
				m.commandInput.SetValue("")
				m.commandInput.Blur()
				m.mode = modeNormal
				return m, nil
			default:
				var inputCmd tea.Cmd
				m.commandInput, inputCmd = m.commandInput.Update(msg)
				return m, inputCmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.exitDialog.Show()
			return m, nil
		case "/":
			m.filterDialog.Show()
			return m, nil
		case ":", "ctrl+p":
			m.mode = modeCommand
			m.commandInput.Focus()
			return m, nil
		case "tab":
			m.focus = (m.focus + 1) % 3
			return m, nil
		case "ctrl+l":
			m.autoScroll = !m.autoScroll
			if m.autoScroll {
				m.scrollToBottom()
			}
			return m, nil
		case "f2":
			m.clearLogs()
			return m, nil
		case "up", "k":
			m.scrollUp()
			return m, nil
		case "down", "j":
			m.scrollDown()
			return m, nil
		case "pgup":
			m.pageUp()
			return m, nil
		case "pgdown", " ":
			m.pageDown()
			return m, nil
		case "home", "g":
			m.scrollToTop()
			return m, nil
		case "end", "G":
			m.scrollToBottom()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateWidths()
		m.maxVisibleLines = m.height - 8
		if m.maxVisibleLines < 5 {
			m.maxVisibleLines = 5
		}
		m.commandInput.Width = m.mainWidth - 10

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case terminalTickMsg:
		m.refreshStats()
		return m, tea.Batch(m.spinner.Tick, terminalTickCmd())

	case logMsg:
		m.logsMutex.Lock()
		m.allLogs = append(m.allLogs, LogEntry(msg))
		if m.maxLogs > 0 && len(m.allLogs) > m.maxLogs {
			m.allLogs = m.allLogs[len(m.allLogs)-m.maxLogs:]
		}
		m.updateFilteredLogs()

		if m.autoScroll {
			logsToShow := m.filteredLogs
			if m.filterText == "" {
				logsToShow = m.allLogs
			}
			logArea := m.maxVisibleLines
			if logArea < 5 {
				logArea = 5
			}
			m.scrollOffset = len(logsToShow) - logArea
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		}
		m.logsMutex.Unlock()
		return m, nil
	}

	return m, cmd
}

func (m *TerminalModel) View() string {
	if m.quitting {
		return ""
	}
	if m.exitDialog.IsActive() {
		return m.exitDialog.View(m.width, m.height)
	}
	if m.filterDialog.IsActive() {
		return m.filterDialog.View(m.width, m.height)
	}

	mainContent := m.renderMainPanel()
	mainBlock := mainPanelStyle.
		Width(m.mainWidth).
		Render(mainContent)

	if m.sidebarHidden {
		return mainBlock
	}

	sidebarContent := m.renderSidebar()
	sidebarBlock := sidebarStyle.
		Width(m.sidebarContentWidth).
		Render(sidebarContent)

	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#44475A")).
		Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarBlock, sep, mainBlock)
}

func (m *TerminalModel) renderSidebar() string {
	cw := m.sidebarContentWidth - 8 // text area inside 4-cell padding on each side
	if cw < 8 {
		cw = 8
	}
	divider := DividerLine.Render(strings.Repeat("─", cw))

	var b strings.Builder

	// Logo banner
	if m.config.Banner != "" {
		for _, line := range strings.Split(m.config.Banner, "\n") {
			line = strings.TrimRight(line, " ")
			if len(line) > cw {
				line = line[:cw]
			}
			b.WriteString(sidebarHeaderStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// App name + version
	header := m.config.AppName
	if m.config.AppVersion != "" {
		header += " v" + m.config.AppVersion
	}
	b.WriteString(sidebarHeaderStyle.Render(header))
	b.WriteString("\n")

	// Status badge
	b.WriteString(focusIndicatorStyle.Render("● RUNNING"))
	b.WriteString("\n")

	// Port / Env
	info := fmt.Sprintf("Port %s   Env %s", m.config.Port, m.config.Env)
	b.WriteString(sidebarValueStyle.Render(info))
	b.WriteString("\n")

	// Uptime
	uptime := time.Since(m.startTime).Round(time.Second).String()
	b.WriteString(sidebarValueStyle.Render("Uptime " + uptime))
	b.WriteString("\n")

	// spacing
	b.WriteString("\n")

	// Divider
	b.WriteString(divider)
	b.WriteString("\n")

	// spacing
	b.WriteString("\n")

	// Resources section
	b.WriteString(m.renderResourcesSection())
	b.WriteString("\n")

	// spacing
	b.WriteString("\n")

	// Components + Services sections
	b.WriteString(m.renderComponentsSection())

	// Pad to full height so the border box stretches to the bottom edge
	targetLines := m.height - 2 // minus top/bottom border
	currentLines := strings.Count(b.String(), "\n") + 1
	if currentLines < targetLines {
		b.WriteString(strings.Repeat("\n", targetLines-currentLines))
	}

	return b.String()
}

// noSidebarBorder is a bubble-table Border with all glyphs set to space,
// producing invisible borders while keeping column alignment.
var noSidebarBorder = table.Border{
	Top:    " ",
	Bottom: " ",
	Left:   " ",
	Right:  " ",
	TopRight:    " ",
	TopLeft:     " ",
	BottomRight: " ",
	BottomLeft:  " ",
	TopJunction:    " ",
	LeftJunction:   " ",
	RightJunction:  " ",
	BottomJunction: " ",
	InnerJunction:  " ",
	InnerDivider:   " ",
}

// sidebarTable2 renders a borderless 2-column table for key-value info.
func (m *TerminalModel) sidebarTable2(labelW int, rows []table.Row) string {
	cw := m.sidebarContentWidth - 8
	valW := cw - labelW - 1
	if valW < 8 {
		valW = 8
	}
	cols := []table.Column{
		table.NewColumn("k", "", labelW).WithStyle(sidebarLabelStyle),
		table.NewColumn("v", "", valW).WithStyle(sidebarValueStyle),
	}
	t := table.New(cols).WithRows(rows).
		WithHeaderVisibility(false).
		Border(noSidebarBorder).
		WithBaseStyle(lipgloss.NewStyle())
	return t.View()
}

// sidebarTable3 renders a borderless 3-column table for icon/name/status lists.
func (m *TerminalModel) sidebarTable3(iconW, nameW int, rows []table.Row) string {
	cw := m.sidebarContentWidth - 8
	statusW := cw - iconW - nameW - 2
	if statusW < 6 {
		statusW = 6
	}
	cols := []table.Column{
		table.NewColumn("i", "", iconW),
		table.NewColumn("n", "", nameW),
		table.NewColumn("s", "", statusW),
	}
	t := table.New(cols).WithRows(rows).
		WithHeaderVisibility(false).
		Border(noSidebarBorder).
		WithBaseStyle(lipgloss.NewStyle())
	return t.View()
}

func (m *TerminalModel) renderResourcesSection() string {
	var lines []string

	sectionTitle := "Resources"
	if m.sidebarContentWidth >= 16 {
		sectionTitle = "─── Resources ───"
	}
	lines = append(lines, sidebarSectionStyle.Render(sectionTitle))

	barW := m.sidebarContentWidth - 20
	if barW < 4 {
		barW = 4
	}

	cpuBar := m.termProgressBar(m.cpuPercent, barW)
	cpuPct := m.percentStyle(m.cpuPercent).Render(fmt.Sprintf("%5.1f%%", m.cpuPercent))
	lines = append(lines, fmt.Sprintf(" %s %s %s", sidebarLabelStyle.Render("CPU"), cpuBar, cpuPct))

	memBar := m.termProgressBar(m.memPercent, barW)
	memPct := m.percentStyle(m.memPercent).Render(fmt.Sprintf("%5.1f%%", m.memPercent))
	lines = append(lines, fmt.Sprintf(" %s %s %s", sidebarLabelStyle.Render("RAM"), memBar, memPct))

	// Mem detail in GiB
	memUsedGiB := fmt.Sprintf("%.1f", float64(m.memUsed)/1024)
	memTotalGiB := fmt.Sprintf("%.1f", float64(m.memTotal)/1024)
	lines = append(lines, fmt.Sprintf("     %s / %s GiB", sidebarValueStyle.Render(memUsedGiB), sidebarDimStyle.Render(memTotalGiB)))

	// Separator
	lines = append(lines, DividerLine.Render(strings.Repeat("─", 38)))

	// Key-value info as 2-column table for proper alignment
	var kvRows []table.Row
	ncpu := runtime.NumCPU()
	kvRows = append(kvRows, table.NewRow(table.RowData{
		"k": "Cores", "v": fmt.Sprintf("%d %s", ncpu, m.percentStyle(m.cpuPercent).Render("●")),
	}))
	kvRows = append(kvRows, table.NewRow(table.RowData{
		"k": "Goroutines", "v": fmt.Sprintf("%d", m.goroutines),
	}))
	kvRows = append(kvRows, table.NewRow(table.RowData{
		"k": "App Mem", "v": fmt.Sprintf("%d MiB", m.appMem),
	}))
	kvRows = append(kvRows, table.NewRow(table.RowData{
		"k": "Plugins", "v": fmt.Sprintf("%d/%d", m.pluginLoaded, m.pluginTotal),
	}))
	if m.hostname != "" {
		hn := m.hostname
		if len(hn) > 24 {
			hn = hn[:24]
		}
		kvRows = append(kvRows, table.NewRow(table.RowData{"k": "Host", "v": hn}))
	}
	if m.cpuModel != "" {
		cm := m.cpuModel
		if len(cm) > 28 {
			cm = cm[:25] + "..."
		}
		kvRows = append(kvRows, table.NewRow(table.RowData{"k": "CPU", "v": cm}))
	}
	kvRows = append(kvRows, table.NewRow(table.RowData{
		"k": "PID", "v": fmt.Sprintf("%d", m.pid),
	}))
	lines = append(lines, m.sidebarTable2(10, kvRows))

	return strings.Join(lines, "\n")
}

func (m *TerminalModel) renderComponentsSection() string {
	cw := m.sidebarContentWidth - 8 // text area inside 4-cell padding on each side
	var lines []string

	compTitle := "Components"
	if cw >= 16 {
		compTitle = "─── Components ───"
	}
	lines = append(lines, sidebarSectionStyle.Render(compTitle))

	if len(m.infraEntries) == 0 {
		lines = append(lines, sidebarDimStyle.Render("  (checking...)"))
	} else {
		var rows []table.Row
		for _, infra := range m.infraEntries {
			color := "#50FA7B"
			status := "connected"
			if !infra.Connected {
				color = "#FF5555"
				status = "error"
			}
			cs := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			name := infra.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			rows = append(rows, table.NewRow(table.RowData{
				"i": table.NewStyledCell("●", cs),
				"n": name,
				"s": table.NewStyledCell(status, cs),
			}))
		}
		lines = append(lines, m.sidebarTable3(2, 20, rows))
	}

	lines = append(lines, "")

	svcTitle := "Services"
	if cw >= 16 {
		svcTitle = "─── Services ───"
	}
	lines = append(lines, sidebarSectionStyle.Render(svcTitle))

	if len(m.serviceEntries) == 0 {
		lines = append(lines, sidebarDimStyle.Render("  (checking...)"))
	} else {
		var rows []table.Row
		for _, svc := range m.serviceEntries {
			color := "#50FA7B"
			icon := "◆"
			status := "running"
			if !svc.Running {
				color = "#6272A4"
				icon = "◇"
				status = "disabled"
			}
			cs := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			name := svc.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			rows = append(rows, table.NewRow(table.RowData{
				"i": table.NewStyledCell(icon, cs),
				"n": name,
				"s": table.NewStyledCell(status, cs),
			}))
		}
		lines = append(lines, m.sidebarTable3(2, 20, rows))
	}

	lines = append(lines, "")

	mwTitle := "Middlewares"
	if cw >= 16 {
		mwTitle = "─── Middlewares ───"
	}
	lines = append(lines, sidebarSectionStyle.Render(mwTitle))

	if len(m.middlewareEntries) == 0 {
		lines = append(lines, sidebarDimStyle.Render("  (checking...)"))
	} else {
		var rows []table.Row
		for _, mw := range m.middlewareEntries {
			color := "#50FA7B"
			icon := "◐"
			status := "on"
			if !mw.Enabled {
				color = "#6272A4"
				icon = "○"
				status = "off"
			}
			cs := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			name := mw.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			rows = append(rows, table.NewRow(table.RowData{
				"i": table.NewStyledCell(icon, cs),
				"n": name,
				"s": table.NewStyledCell(status, cs),
			}))
		}
		lines = append(lines, m.sidebarTable3(2, 20, rows))
	}

	return strings.Join(lines, "\n")
}

func (m *TerminalModel) renderMainPanel() string {
	var b strings.Builder

	logArea := m.maxVisibleLines
	if logArea < 5 {
		logArea = 5
	}

	b.WriteString(mainPanelStyle.Render("▪ Live Logs"))
	b.WriteString("\n")

	b.WriteString(DividerLine.Render(strings.Repeat("─", m.mainWidth-4)))
	b.WriteString("\n")

	logLines := m.renderLogEntries()
	if len(logLines) > logArea {
		start := m.scrollOffset
		if start < 0 {
			start = 0
		}
		if start >= len(logLines) {
			start = len(logLines) - 1
		}
		end := start + logArea
		if end > len(logLines) {
			end = len(logLines)
		}
		logLines = logLines[start:end]
	}

	for _, line := range logLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	for i := len(logLines); i < logArea; i++ {
		b.WriteString("\n")
	}

	b.WriteString("\n")

	b.WriteString(m.renderCommandInput())
	b.WriteString("\n")

	b.WriteString(m.renderFooter())

	return b.String()
}

func (m *TerminalModel) renderCommandInput() string {
	fullWidth := m.mainWidth - 4
	if fullWidth < 20 {
		fullWidth = 20
	}

	if m.mode == modeCommand {
		return commandBoxActiveStyle.Width(fullWidth).Render(m.commandInput.View())
	}

	placeholder := ": type command... (ctrl+p)"
	if m.focus == focusCommand {
		return commandBoxActiveStyle.Width(fullWidth).Render(sidebarDimStyle.Render(placeholder))
	}
	return commandBoxStyle.Width(fullWidth).Render(sidebarDimStyle.Render(placeholder))
}

func (m *TerminalModel) renderFooter() string {
	var parts []string

	if m.mode == modeCommand {
		parts = []string{"enter: exec", "esc: cancel"}
	} else {
		parts = []string{"/: filter", "ctrl+l: scroll", "tab: focus", "F2: clear", "ctrl+c: exit"}
		if m.focus == focusSidebar {
			parts = append(parts, "sidebar")
		} else if m.focus == focusLogs {
			parts = append(parts, "logs")
		} else if m.focus == focusCommand {
			parts = append(parts, "command")
		}
	}

	return sidebarDimStyle.Render(strings.Join(parts, " ● "))
}

func (m *TerminalModel) renderLogEntries() []string {
	var lines []string

	lw := m.logWidth
	if lw < 40 {
		lw = 40
	}

	m.logsMutex.RLock()
	defer m.logsMutex.RUnlock()

	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}

		if len(logsToShow) == 0 {
		lines = append(lines, sidebarDimStyle.Render("  Waiting for logs..."))
	} else {
		for _, log := range logsToShow {
			ls := m.levelStyle(log.Level)
			timeStr := log.Time.Format("15:04:05")
			icon := m.levelIcon(log.Level)

			maxMsgLen := lw - 22
			if maxMsgLen < 20 {
				maxMsgLen = 20
			}
			msg := log.Message
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-3] + "..."
			}

			levelPad := ""
			line := fmt.Sprintf("  %s %s%s %s",
				sidebarDimStyle.Render(timeStr),
				ls.Render(icon),
				levelPad,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")).Render(msg),
			)
			lines = append(lines, line)
		}
	}

	return lines
}

func (m *TerminalModel) levelIcon(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "⚙"
	case "info":
		return "●"
	case "warn", "warning":
		return "⚠"
	case "error":
		return "✗"
	case "fatal":
		return "‼"
	default:
		return "•"
	}
}

func (m *TerminalModel) levelStyle(level string) lipgloss.Style {
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

func (m *TerminalModel) executeCommand(raw string) tea.Cmd {
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return nil
	}
	switch cmd {
	case ":help":
		return m.logCmd("info", "Commands: :help :clear :stats :services :infra")
	case ":clear":
		m.clearLogs()
		return nil
	case ":stats":
		return m.logCmd("info", fmt.Sprintf("CPU %.1f%% | RAM %.1f%% (%d/%d MiB) | Goroutines %d | Host %s",
			m.cpuPercent, m.memPercent, m.memUsed, m.memTotal, m.goroutines, m.hostname))
	case ":services":
		running := 0
		for _, s := range m.serviceEntries {
			if s.Running {
				running++
			}
		}
		return m.logCmd("info", fmt.Sprintf("Services: %d total, %d running", len(m.serviceEntries), running))
	case ":infra":
		connected := 0
		for _, i := range m.infraEntries {
			if i.Connected {
				connected++
			}
		}
		return m.logCmd("info", fmt.Sprintf("Infra: %d total, %d connected", len(m.infraEntries), connected))
	default:
		return m.logCmd("warn", "Unknown command: "+cmd+" (try :help)")
	}
}

func (m *TerminalModel) logCmd(level, msg string) tea.Cmd {
	return func() tea.Msg {
		return logMsg{Time: time.Now(), Level: level, Message: msg}
	}
}

func (m *TerminalModel) AddLog(level, message string) {
	if m.program != nil {
		m.program.Send(logMsg{
			Time:    time.Now(),
			Level:   level,
			Message: message,
		})
	}
}

func (m *TerminalModel) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *TerminalModel) updateFilteredLogs() {
	if m.filterText == "" {
		m.filteredLogs = m.allLogs
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

func (m *TerminalModel) scrollDown() {
	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}
	if m.scrollOffset < len(logsToShow)-m.maxVisibleLines {
		m.scrollOffset++
		m.autoScroll = false
	}
}

func (m *TerminalModel) scrollUp() {
	if m.scrollOffset > 0 {
		m.scrollOffset--
		m.autoScroll = false
	}
}

func (m *TerminalModel) pageDown() {
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
	m.autoScroll = false
}

func (m *TerminalModel) pageUp() {
	m.scrollOffset -= m.maxVisibleLines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = false
}

func (m *TerminalModel) scrollToTop() {
	m.scrollOffset = 0
	m.autoScroll = false
}

func (m *TerminalModel) scrollToBottom() {
	logsToShow := m.filteredLogs
	if m.filterText == "" {
		logsToShow = m.allLogs
	}
	m.scrollOffset = len(logsToShow) - m.maxVisibleLines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = true
}

func (m *TerminalModel) clearLogs() {
	m.logsMutex.Lock()
	defer m.logsMutex.Unlock()

	m.allLogs = make([]LogEntry, 0)
	m.filteredLogs = make([]LogEntry, 0)
	m.scrollOffset = 0
	m.filterText = ""
}

func (m *TerminalModel) calculateWidths() {
	// Auto-hide sidebar when terminal is too narrow for comfortable side-by-side layout
	m.sidebarHidden = m.width < 100

	if m.sidebarHidden {
		m.sidebarWidth = 0
		m.sidebarContentWidth = 0
		m.mainWidth = m.width
		if m.mainWidth < 40 {
			m.mainWidth = 40
		}
	} else {
		sidebarWidth := m.width * 30 / 100
		if sidebarWidth < 34 {
			sidebarWidth = 34
		}
		if sidebarWidth > m.width/2 {
			sidebarWidth = m.width / 2
		}
		m.sidebarWidth = sidebarWidth
		m.sidebarContentWidth = sidebarWidth
		if m.sidebarContentWidth < 16 {
			m.sidebarContentWidth = 16
		}
		// -1 reserves the thin "│" separator column between sidebar and main panel
		m.mainWidth = m.width - m.sidebarWidth - 1
		if m.mainWidth < 40 {
			m.mainWidth = 40
		}
	}
	m.logWidth = m.mainWidth - 4
	if m.logWidth < 40 {
		m.logWidth = 40
	}
}

func (m *TerminalModel) refreshStats() {
	if v, err := mem.VirtualMemory(); err == nil {
		m.memPercent = v.UsedPercent
		m.memUsed = v.Used / 1024 / 1024
		m.memTotal = v.Total / 1024 / 1024
	}
	if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
		m.cpuPercent = c[0]
	}
	m.goroutines = runtime.NumGoroutine()

	if m.hostname == "" {
		if info, err := utils.GetNetworkInfo(); err == nil {
			m.hostname = info["hostname"]
		}
	}

	if m.cpuModel == "" {
		if info, err := cpu.Info(); err == nil && len(info) > 0 {
			m.cpuModel = info[0].ModelName
		}
	}

	m.appMem = utils.GetMemSelf()

	reg := infrastructure.GetGlobalRegistry()
	all := reg.GetAll()
	infraEntries := make([]InfraEntry, 0, len(all))
	for name, comp := range all {
		status := comp.GetStatus()
		connected, _ := status["connected"].(bool)
		infraEntries = append(infraEntries, InfraEntry{Name: name, Connected: connected})
	}
	sort.Slice(infraEntries, func(i, j int) bool {
		return infraEntries[i].Name < infraEntries[j].Name
	})
	m.infraEntries = infraEntries

	if comp, ok := reg.Get("plugins"); ok {
		status := comp.GetStatus()
		if t, ok := status["total"].(int); ok {
			m.pluginTotal = t
		}
		if l, ok := status["loaded"].(int); ok {
			m.pluginLoaded = l
		}
	}

	mwNames := middleware.GetGlobalMiddlewareRegistry().GetNames()
	sort.Strings(mwNames)
	mwEntries := make([]MiddlewareEntry, 0, len(mwNames))
	for _, name := range mwNames {
		mwEntries = append(mwEntries, MiddlewareEntry{
			Name:    name,
			Enabled: middleware.GetGlobalMiddlewareRegistry().IsEnabled(name),
		})
	}
	m.middlewareEntries = mwEntries

	factories := registry.GetServiceFactories()
	svcNames := make([]string, 0, len(factories))
	for name := range factories {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)
	serviceEntries := make([]ServiceEntry, 0, len(svcNames))
	for _, name := range svcNames {
		running := registry.GetService(name) != nil
		serviceEntries = append(serviceEntries, ServiceEntry{Name: name, Running: running})
	}
	m.serviceEntries = serviceEntries
}

func (m *TerminalModel) termProgressBar(percent float64, width int) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	bar := m.percentStyle(percent).Render(strings.Repeat("█", filled)) +
		sidebarDimStyle.Render(strings.Repeat("░", empty))
	return bar
}

// ponytail: pastel progress stops at 3 thresholds — add more if smoother gradient needed
func (m *TerminalModel) percentStyle(percent float64) lipgloss.Style {
	switch {
	case percent < 50:
		return dashPastelGood
	case percent < 80:
		return dashPastelWarn
	default:
		return dashPastelBad
	}
}

type TerminalTUI struct {
	model   *TerminalModel
	program *tea.Program
}

func NewTerminalTUI(cfg LiveConfig) *TerminalTUI {
	model := NewTerminalModel(cfg)
	return &TerminalTUI{model: model}
}

func (t *TerminalTUI) Start() {
	t.program = tea.NewProgram(t.model, tea.WithAltScreen())
	t.model.SetProgram(t.program)
	go func() {
		_, _ = t.program.Run()
	}()
}

func (t *TerminalTUI) Stop() {
	if t.program != nil {
		utils.ClearScreen()
		t.program.Quit()
		os.Exit(0)
	}
}

func (t *TerminalTUI) AddLog(level, message string) {
	t.model.AddLog(level, message)
}

func (t *TerminalTUI) Write(p []byte) (n int, err error) {
	line := strings.TrimSpace(string(p))
	if line != "" {
		level, message := parseLogLine(line)
		if message != "" {
			t.AddLog(level, message)
		}
	}
	return len(p), nil
}
