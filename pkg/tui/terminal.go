package tui

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/tui/template"
	"stackyrd/pkg/utils"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
	"github.com/muesli/reflow/wordwrap"
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
	Enabled   bool
}

type ServiceEntry struct {
	Name     string
	Running  bool
	Failed   bool // enabled by config but could not start (missing infra dep)
	Disabled bool // disabled by config
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
	sidebarOffset   int
	maxVisibleLines int
	autoScroll      bool
	startTime       time.Time
	width           int
	height          int
	quitting        bool
	maxLogs         int
	program         atomic.Pointer[tea.Program]

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

	cpuModel            string
	pid                 int
	appMem              uint64
	sidebarWidth        int
	sidebarContentWidth int
	mainWidth           int
	logWidth            int
	sidebarHidden       bool
}

type terminalTickMsg time.Time

func terminalTickCmd() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		return terminalTickMsg(t)
	})
}

// infoDialogMsg requests the info dialog to be shown with styled content.
func NewTerminalModel(cfg LiveConfig) *TerminalModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary")))

	ti := textinput.New()
	ti.Placeholder = "Type command (help, themes, theme <name>...; ':' optional)"
	ti.CharLimit = 500
	ti.Width = 60
	ti.Prompt = ": "
	ti.PromptStyle = commandPromptStyle()
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary")))

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
	case tea.MouseMsg:
		// Wheel scrolls the pane under the cursor so sidebar and logs scroll
		// independently — scrolling one never moves the other.
		if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
			up := msg.Type == tea.MouseWheelUp
			if msg.X >= 0 && msg.X < m.sidebarWidth {
				if up {
					m.sidebarUp()
				} else {
					m.sidebarDown()
				}
			} else {
				if up {
					m.scrollUp()
				} else {
					m.scrollDown()
				}
			}
		}
		return m, nil

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
			if m.focus == focusSidebar {
				m.sidebarUp()
			} else {
				m.scrollUp()
			}
			return m, nil
		case "down", "j":
			if m.focus == focusSidebar {
				m.sidebarDown()
			} else {
				m.scrollDown()
			}
			return m, nil
		case "pgup":
			if m.focus == focusSidebar {
				m.sidebarPageUp()
			} else {
				m.pageUp()
			}
			return m, nil
		case "pgdown", " ":
			if m.focus == focusSidebar {
				m.sidebarPageDown()
			} else {
				m.pageDown()
			}
			return m, nil
		case "home", "g":
			if m.focus == focusSidebar {
				m.sidebarScrollToTop()
			} else {
				m.scrollToTop()
			}
			return m, nil
		case "end", "G":
			if m.focus == focusSidebar {
				m.sidebarScrollToBottom()
			} else {
				m.scrollToBottom()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateWidths()
		m.maxVisibleLines = m.height - 5
		if m.maxVisibleLines < 5 {
			m.maxVisibleLines = 5
		}
		m.commandInput.Width = m.mainWidth - 10
		// clamp scroll offset after height change so logs don't show "weird" empty rows
		maxOff := m.logLineCount() - m.maxVisibleLines
		if maxOff < 0 {
			maxOff = 0
		}
		if m.scrollOffset > maxOff {
			m.scrollOffset = maxOff
		}
		// clamp sidebar offset after height change
		if m.sidebarOffset > m.height {
			m.sidebarOffset = m.height
		}
		if m.sidebarOffset < 0 {
			m.sidebarOffset = 0
		}

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
		m.logsMutex.Unlock()
		return m, nil
	}

	return m, cmd
}

// View renders the full TUI composed of three components:
// Sidebar (left) | LogView + CommandBar (right, stacked vertically)
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

	// ── Right column: LogView on top, CommandBar on bottom ──
	logBlock := m.renderLogView()
	cmdBlock := m.renderCommandBar()

	rightBlock := lipgloss.JoinVertical(lipgloss.Top, logBlock, cmdBlock)

	if m.sidebarHidden {
		return rightBlock
	}

	// ── Left column: Sidebar ──
	sidebarContent := m.renderSidebar()
	sidebarBlock := sidebarStyle().
		Width(m.sidebarContentWidth).
		Render(sidebarContent)

	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color(TC("dim"))).
		Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarBlock, sep, rightBlock)
}

func (m *TerminalModel) renderSidebar() string {
	raw := m.renderSidebarContent()
	lines := strings.Split(raw, "\n")
	total := len(lines)
	maxH := m.height
	if maxH < 1 {
		maxH = 1
	}
	off := m.sidebarOffset
	if off > total-maxH {
		off = total - maxH
	}
	if off < 0 {
		off = 0
	}
	end := off + maxH
	if end > total {
		end = total
	}
	return strings.Join(lines[off:end], "\n")
}

func (m *TerminalModel) renderSidebarContent() string {
	cw := m.sidebarContentWidth - 8 // text area inside 4-cell padding on each side
	if cw < 8 {
		cw = 8
	}
	divider := DividerLine().Render()

	var b strings.Builder

	// Logo banner
	if m.config.Banner != "" {
		for _, line := range strings.Split(m.config.Banner, "\n") {
			line = strings.TrimRight(line, " ")
			if len([]rune(line)) > cw {
				line = string([]rune(line)[:cw])
			}
			b.WriteString(sidebarHeaderStyle().Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// App name + version
	header := m.config.AppName
	if m.config.AppVersion != "" {
		header += " v" + m.config.AppVersion
	}
	b.WriteString(sidebarHeaderStyle().Render(header))
	b.WriteString("\n")

	// Status badge
	b.WriteString(focusIndicatorStyle().Render("● RUNNING"))
	b.WriteString("\n")

	// Port / Env
	info := fmt.Sprintf("Port %s   Env %s", m.config.Port, m.config.Env)
	b.WriteString(sidebarValueStyle().Render(info))
	b.WriteString("\n")

	// Uptime
	uptime := time.Since(m.startTime).Round(time.Second).String()
	b.WriteString(sidebarValueStyle().Render("Uptime " + uptime))
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

	// Components + Services + Middlewares sections
	b.WriteString(m.renderComponentsSection())

	return b.String()
}

// noSidebarBorder is a bubble-table Border with all glyphs set to space,
// producing invisible borders while keeping column alignment.
var noSidebarBorder = table.Border{
	Top:            " ",
	Bottom:         " ",
	Left:           " ",
	Right:          " ",
	TopRight:       " ",
	TopLeft:        " ",
	BottomRight:    " ",
	BottomLeft:     " ",
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
		table.NewColumn("k", "", labelW).WithStyle(sidebarLabelStyle()),
		table.NewColumn("v", "", valW).WithStyle(sidebarValueStyle()),
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
	lines = append(lines, sidebarSectionStyle().Render(sectionTitle))

	barW := m.sidebarContentWidth - 20
	if barW < 4 {
		barW = 4
	}

	cpuBar := m.termProgressBar(m.cpuPercent, barW)
	cpuPct := m.percentStyle(m.cpuPercent).Render(fmt.Sprintf("%5.1f%%", m.cpuPercent))
	lines = append(lines, fmt.Sprintf(" %s %s %s", sidebarLabelStyle().Render("CPU"), cpuBar, cpuPct))

	memBar := m.termProgressBar(m.memPercent, barW)
	memPct := m.percentStyle(m.memPercent).Render(fmt.Sprintf("%5.1f%%", m.memPercent))
	lines = append(lines, fmt.Sprintf(" %s %s %s", sidebarLabelStyle().Render("RAM"), memBar, memPct))

	// Mem detail in GiB
	memUsedGiB := fmt.Sprintf("%.1f", float64(m.memUsed)/1024)
	memTotalGiB := fmt.Sprintf("%.1f", float64(m.memTotal)/1024)
	lines = append(lines, fmt.Sprintf("     %s / %s GiB", sidebarValueStyle().Render(memUsedGiB), sidebarDimStyle().Render(memTotalGiB)))

	// Separator
	lines = append(lines, DividerLine().Render(strings.Repeat("─", 38)))

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
	if m.hostname != "" {
		hn := m.hostname
		if runes := []rune(hn); len(runes) > 24 {
			hn = string(runes[:24])
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
	lines = append(lines, sidebarSectionStyle().Render(compTitle))

	if len(m.infraEntries) == 0 {
		lines = append(lines, sidebarDimStyle().Render("  (checking...)"))
	} else {
		var rows []table.Row
		maxVisible := 4
		for i, infra := range m.infraEntries {
			if i >= maxVisible {
				break
			}
			color := TC("success")
			icon := "●"
			status := "connected"
			if !infra.Connected && infra.Enabled {
				color = TC("error")
				icon = "✗"
				status = "failed"
			} else if !infra.Enabled {
				color = TC("dim")
				icon = "○"
				status = "disabled"
			}
			cs := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			name := infra.Name
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
		if len(m.infraEntries) > maxVisible {
			remaining := len(m.infraEntries) - maxVisible
			lines = append(lines, sidebarDimStyle().Render(fmt.Sprintf("  +%d more hidden", remaining)))
		}
	}

	lines = append(lines, "")

	svcTitle := "Services"
	if cw >= 16 {
		svcTitle = "─── Services ───"
	}
	lines = append(lines, sidebarSectionStyle().Render(svcTitle))

	if len(m.serviceEntries) == 0 {
		lines = append(lines, sidebarDimStyle().Render("  (checking...)"))
	} else {
		var rows []table.Row
		maxVisible := 4
		for i, svc := range m.serviceEntries {
			if i >= maxVisible {
				break
			}
			color := TC("success")
			icon := "◆"
			status := "running"
			switch {
			case svc.Failed:
				color = TC("error")
				icon = "✗"
				status = "failed"
			case svc.Disabled:
				color = TC("dim")
				icon = "◇"
				status = "disabled"
			case !svc.Running:
				color = TC("dim")
				icon = "◇"
				status = "stopped"
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
		if len(m.serviceEntries) > maxVisible {
			remaining := len(m.serviceEntries) - maxVisible
			lines = append(lines, sidebarDimStyle().Render(fmt.Sprintf("  +%d more hidden", remaining)))
		}
	}

	lines = append(lines, "")

	mwTitle := "Middlewares"
	if cw >= 16 {
		mwTitle = "─── Middlewares ───"
	}
	lines = append(lines, sidebarSectionStyle().Render(mwTitle))

	if len(m.middlewareEntries) == 0 {
		lines = append(lines, sidebarDimStyle().Render("  (checking...)"))
	} else {
		var rows []table.Row
		for _, mw := range m.middlewareEntries {
			color := TC("success")
			icon := "◐"
			status := "on"
			if !mw.Enabled {
				color = TC("dim")
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

func (m *TerminalModel) renderLogView() string {
	var b strings.Builder

	logArea := m.maxVisibleLines
	if logArea < 5 {
		logArea = 5
	}

	// Header is written plain; the outer mainPanelStyle().Width(...) render at
	// the end applies color + padding uniformly. Double-rendering it here added
	// a second 4-col padding (8 spaces before "▪") and a stray colored gap.
	b.WriteString("▪ Live Logs")
	b.WriteString("\n\n")

	logLines := m.renderLogEntries()
	if len(logLines) > logArea {
		if m.autoScroll {
			// Follow the newest logs: show the last logArea physical lines so
			// word-wrapped entries never shift the viewport.
			logLines = logLines[len(logLines)-logArea:]
		} else {
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
	}

	// Join log lines without a trailing newline so the panel is exactly
	// header + blank + logArea lines tall (a trailing "\n" added one extra
	// line that overflowed the terminal and misaligned the sidebar).
	for i, line := range logLines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}

	for i := len(logLines); i < logArea; i++ {
		b.WriteString("\n")
	}

	return mainPanelStyle().Width(m.mainWidth).Render(b.String())
}

func (m *TerminalModel) renderCommandBar() string {
	var b strings.Builder
	b.WriteString(m.renderCommandInput())
	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return mainPanelStyle().Width(m.mainWidth).Render(b.String())
}

func (m *TerminalModel) renderCommandInput() string {
	fullWidth := m.mainWidth - 4
	if fullWidth < 20 {
		fullWidth = 20
	}

	if m.mode == modeCommand {
		return commandBoxActiveStyle().Width(fullWidth).Render(m.commandInput.View())
	}

	placeholder := ": type command... (ctrl+p)"
	if m.focus == focusCommand {
		return commandBoxActiveStyle().Width(fullWidth).Render(sidebarDimStyle().Render(placeholder))
	}
	return commandBoxStyle().Width(fullWidth).Render(sidebarDimStyle().Render(placeholder))
}

func (m *TerminalModel) renderFooter() string {
	var parts []string

	if m.mode == modeCommand {
		parts = []string{"enter: exec", "esc: cancel"}
	} else {
		parts = []string{"/: filter", "ctrl+l: scroll", "tab: focus", "F2: clear", "ctrl+c: exit", "wheel: scroll pane"}
		if m.focus == focusSidebar {
			parts = append(parts, "sidebar")
		} else if m.focus == focusLogs {
			parts = append(parts, "logs")
		} else if m.focus == focusCommand {
			parts = append(parts, "command")
		}
	}

	return sidebarDimStyle().Render(strings.Join(parts, " ● "))
}

// wrapLogMessage returns the display lines for a log message. Normal messages
// are word-wrapped for readability; long unbroken tokens (serialized errors,
// stack traces, URLs) cannot be wrapped and are flattened to a single
// truncated line so they don't flood the log view.
func wrapLogMessage(msg string, maxLen int) []string {
	if maxLen < 20 {
		maxLen = 20
	}
	// A single token longer than the wrap width cannot be word-wrapped (e.g. a
	// serialized JSON error). Detect it and print the message plainly instead.
	for _, tok := range strings.Fields(msg) {
		if len(tok) > maxLen {
			return []string{flatLogLine(msg, maxLen)}
		}
	}
	return strings.Split(wordwrap.String(msg, maxLen), "\n")
}

// flatLogLine flattens newlines and truncates to a single log line.
func flatLogLine(msg string, maxLen int) string {
	flat := strings.ReplaceAll(msg, "\n", " ")
	if len(flat) > maxLen {
		flat = flat[:maxLen-3] + "..."
	}
	return flat
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
		lines = append(lines, sidebarDimStyle().Render("  Waiting for logs..."))
	} else {
		for _, log := range logsToShow {
			ls := m.levelStyle(log.Level)
			timeStr := log.Time.Format("15:04:05")
			icon := m.levelIcon(log.Level)

			maxMsgLen := lw - 22
			if maxMsgLen < 20 {
				maxMsgLen = 20
			}
			// Word-wrap instead of truncating so long messages stay readable.
			// Long error blobs are flattened to one line by wrapLogMessage.
			msgLines := wrapLogMessage(log.Message, maxMsgLen)

			prefix := fmt.Sprintf("  %s %s", sidebarDimStyle().Render(timeStr), ls.Render(icon))
			indent := strings.Repeat(" ", 14)
			for i, ml := range msgLines {
				styled := lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text"))).Render(ml)
				if i == 0 {
					lines = append(lines, prefix+" "+styled)
				} else {
					lines = append(lines, indent+styled)
				}
			}
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("secondary")))
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("success")))
	case "warn", "warning":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("warning")))
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("error")))
	case "fatal":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("error"))).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("text")))
	}
}

func (m *TerminalModel) executeCommand(raw string) tea.Cmd {
	cmd := strings.TrimSpace(raw)
	// Accept commands with or without the leading colon (":help" or "help").
	cmd = strings.TrimPrefix(cmd, ":")
	if cmd == "" {
		return nil
	}
	switch cmd {
	case "help":
		return m.logCmd("info", "Commands: help, clear, stats, gc, version, uptime, services, infra [name], mw, deps, endpoints, list, themes, theme <name> (colon optional)")
	case "clear":
		m.clearLogs()
		return nil
	case "stats":
		return m.logCmd("info", fmt.Sprintf("CPU %.1f%% | RAM %.1f%% (%d/%d MiB) | Goroutines %d | Host %s",
			m.cpuPercent, m.memPercent, m.memUsed, m.memTotal, m.goroutines, m.hostname))
	case "gc":
		return m.runGC()
	case "version":
		return m.logCmd("info", fmt.Sprintf("%s v%s | env: %s | http://localhost:%s | up %s",
			m.config.AppName, m.config.AppVersion, m.config.Env, m.config.Port,
			time.Since(m.startTime).Round(time.Second)))
	case "uptime":
		return m.logCmd("info", "Uptime: "+time.Since(m.startTime).Round(time.Second).String())
	case "services":
		running := 0
		failed := 0
		for _, s := range m.serviceEntries {
			if s.Running {
				running++
			}
			if s.Failed {
				failed++
			}
		}
		return m.logCmd("info", fmt.Sprintf("Services: %d total, %d running, %d failed",
			len(m.serviceEntries), running, failed))
	case "infra":
		connected := 0
		for _, i := range m.infraEntries {
			if i.Connected {
				connected++
			}
		}
		return m.logCmd("info", fmt.Sprintf("Infra: %d total, %d connected", len(m.infraEntries), connected))
	case "mw", "middleware", "middlewares":
		return m.listMiddleware()
	case "deps":
		return m.listDeps()
	case "endpoints":
		return m.listEndpoints()
	case "list", "ls":
		return m.listAll()
	case "themes":
		return m.listThemes()
	case "redis", "postgres", "mongo", "kafka", "grafana", "minio", "cron", "webhook", "websocket", "afero":
		return m.infraStatus(cmd)
	default:
		if name, ok := strings.CutPrefix(cmd, "theme"); ok {
			return m.handleThemeCommand(strings.TrimSpace(name))
		}
		if name, ok := strings.CutPrefix(cmd, "infra"); ok && strings.TrimSpace(name) != "" {
			return m.infraStatus(strings.TrimSpace(name))
		}
		return m.logCmd("warn", "Unknown command: "+cmd+" (try help)")
	}
}

// listMiddleware prints middleware names and their enabled state.
func (m *TerminalModel) listMiddleware() tea.Cmd {
	reg := middleware.GetGlobalMiddlewareRegistry()
	names := reg.GetNames()
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Middleware (%d):", len(names)))
	for _, n := range names {
		state := "enabled"
		if !reg.IsEnabled(n) {
			state = "disabled"
		}
		sb.WriteString("\n  " + n + " (" + state + ")")
	}
	return m.logCmd("info", sb.String())
}

// listDeps prints a one-line dependency summary across all three layers.
func (m *TerminalModel) listDeps() tea.Cmd {
	infraConnected := 0
	for _, i := range m.infraEntries {
		if i.Connected {
			infraConnected++
		}
	}
	svcRunning := 0
	svcFailed := 0
	for _, s := range m.serviceEntries {
		if s.Running {
			svcRunning++
		}
		if s.Failed {
			svcFailed++
		}
	}
	mwReg := middleware.GetGlobalMiddlewareRegistry()
	mwNames := mwReg.GetNames()
	mwEnabled := 0
	for _, n := range mwNames {
		if mwReg.IsEnabled(n) {
			mwEnabled++
		}
	}
	return m.logCmd("info", fmt.Sprintf(
		"Infra %d/%d connected | Services %d/%d running (%d failed) | Middleware %d/%d enabled",
		infraConnected, len(m.infraEntries), svcRunning, len(m.serviceEntries), svcFailed,
		mwEnabled, len(mwNames)))
}

// listEndpoints prints every service endpoint as plain text.
func (m *TerminalModel) listEndpoints() tea.Cmd {
	factories := registry.GetServiceFactories()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("Endpoints:")
	any := false
	for _, name := range names {
		svc, ok := registry.GetService(name).(endpointProvider)
		if !ok {
			continue
		}
		for _, ep := range svc.Endpoints() {
			any = true
			sb.WriteString("\n  " + svc.Name() + " " + ep)
		}
	}
	if !any {
		sb.WriteString("\n  none")
	}
	return m.logCmd("info", sb.String())
}

// infraStatus prints the full status map of one infrastructure component.
func (m *TerminalModel) infraStatus(name string) tea.Cmd {
	comp, ok := infrastructure.GetGlobalRegistry().Get(name)
	if !ok {
		return m.logCmd("warn", "No infrastructure component '"+name+"' (try infra)")
	}

	status := comp.GetStatus()
	var sb strings.Builder
	sb.WriteString("Component: " + name)
	for k, v := range status {
		sb.WriteString(fmt.Sprintf("\n  %s=%v", k, v))
	}
	return m.logCmd("info", sb.String())
}

// listThemes prints available themes as plain log text.
func (m *TerminalModel) listThemes() tea.Cmd {
	available := AvailableThemeNames()
	sort.Strings(available)
	current := GetThemeName()

	var sb strings.Builder
	sb.WriteString("Available themes (" + current + " active):")
	for _, name := range available {
		if name == current {
			sb.WriteString("\n  " + name + " (active)")
		} else {
			sb.WriteString("\n  " + name)
		}
	}
	sb.WriteString("\nUsage: theme <name>")
	return m.logCmd("info", sb.String())
}

// listAll prints services, infrastructure components, and endpoints as plain
// log text.
func (m *TerminalModel) listAll() tea.Cmd {
	var sb strings.Builder

	sb.WriteString("Services:")
	if len(m.serviceEntries) == 0 {
		sb.WriteString("\n  none")
	} else {
		for _, s := range m.serviceEntries {
			status := "stopped"
			switch {
			case s.Running:
				status = "running"
			case s.Failed:
				status = "failed"
			case s.Disabled:
				status = "disabled"
			}
			sb.WriteString("\n  " + s.Name + " (" + status + ")")
		}
	}

	sb.WriteString("\n\nComponents:")
	if len(m.infraEntries) == 0 {
		sb.WriteString("\n  none")
	} else {
		for _, e := range m.infraEntries {
			status := "disconnected"
			if e.Connected {
				status = "connected"
			}
			sb.WriteString("\n  " + e.Name + " (" + status + ")")
		}
	}

	sb.WriteString("\n\nEndpoints:")
	factories := registry.GetServiceFactories()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	any := false
	for _, name := range names {
		svc, ok := registry.GetService(name).(endpointProvider)
		if !ok {
			continue
		}
		for _, ep := range svc.Endpoints() {
			any = true
			sb.WriteString("\n  " + svc.Name() + " " + ep)
		}
	}
	if !any {
		sb.WriteString("\n  none")
	}

	return m.logCmd("info", sb.String())
}

// endpointProvider is satisfied by services that expose their routes.
type endpointProvider interface {
	Name() string
	Endpoints() []string
}

// runGC forces a garbage collection cycle and reports heap before/after.
func (m *TerminalModel) runGC() tea.Cmd {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	runtime.GC()
	runtime.ReadMemStats(&after)

	usedBefore := before.HeapAlloc / (1 << 20)
	usedAfter := after.HeapAlloc / (1 << 20)
	return m.logCmd("info", fmt.Sprintf("GC forced: heap %d MiB -> %d MiB, cycles %d", usedBefore, usedAfter, after.NumGC))
}

// handleThemeCommand switches the live theme and persists the change to
// config.yaml so it survives a restart.
func (m *TerminalModel) handleThemeCommand(name string) tea.Cmd {
	if name == "" || name == "list" {
		return m.listThemes()
	}
	if !ThemeExists(name) {
		return m.logCmd("warn", "Unknown theme: "+name+" (try :themes)")
	}

	SetThemeName(name)
	m.applyTheme()

	msg := "Theme switched to " + name
	if err := config.SaveTheme(name); err != nil {
		msg += " (not persisted: " + err.Error() + ")"
	} else {
		msg += " (persisted to config.yaml)"
	}
	return m.logCmd("info", msg)
}

// applyTheme re-applies colors that are baked into the model at construction
// time (spinner, command cursor) after a runtime theme switch.
func (m *TerminalModel) applyTheme() {
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary")))
	m.commandInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(TC("primary")))
}

func (m *TerminalModel) logCmd(level, msg string) tea.Cmd {
	return func() tea.Msg {
		return logMsg{Time: time.Now(), Level: level, Message: msg}
	}
}

func (m *TerminalModel) AddLog(level, message string) {
	if p := m.program.Load(); p != nil {
		p.Send(logMsg{
			Time:    time.Now(),
			Level:   level,
			Message: message,
		})
	}
}

func (m *TerminalModel) SetProgram(p *tea.Program) {
	m.program.Store(p)
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

// logLineCount returns the number of physical lines the log view renders.
// Word-wrapped entries occupy more than one line, so scroll bounds must be
// based on physical lines, not log-entry counts.
func (m *TerminalModel) logLineCount() int {
	return len(m.renderLogEntries())
}

func (m *TerminalModel) scrollDown() {
	if m.scrollOffset < m.logLineCount()-m.maxVisibleLines {
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
	m.scrollOffset += m.maxVisibleLines
	maxOffset := m.logLineCount() - m.maxVisibleLines
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
	m.scrollOffset = m.logLineCount() - m.maxVisibleLines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.autoScroll = true
}

func (m *TerminalModel) sidebarLines() []string {
	raw := m.renderSidebarContent()
	return strings.Split(raw, "\n")
}

func (m *TerminalModel) sidebarMaxOffset() int {
	lines := m.sidebarLines()
	maxOff := len(lines) - m.height + 4
	if maxOff < 0 {
		maxOff = 0
	}
	return maxOff
}

func (m *TerminalModel) sidebarUp() {
	if m.sidebarOffset > 0 {
		m.sidebarOffset--
	}
}

func (m *TerminalModel) sidebarDown() {
	if m.sidebarOffset < m.sidebarMaxOffset() {
		m.sidebarOffset++
	}
}

func (m *TerminalModel) sidebarPageUp() {
	m.sidebarOffset -= m.height / 2
	if m.sidebarOffset < 0 {
		m.sidebarOffset = 0
	}
}

func (m *TerminalModel) sidebarPageDown() {
	m.sidebarOffset += m.height / 2
	if m.sidebarOffset > m.sidebarMaxOffset() {
		m.sidebarOffset = m.sidebarMaxOffset()
	}
}

func (m *TerminalModel) sidebarScrollToTop() {
	m.sidebarOffset = 0
}

func (m *TerminalModel) sidebarScrollToBottom() {
	m.sidebarOffset = m.sidebarMaxOffset()
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
	infraByName := make(map[string]InfraEntry, len(all))
	for name, comp := range all {
		status := comp.GetStatus()
		connected, _ := status["connected"].(bool)
		infraByName[name] = InfraEntry{Name: name, Connected: connected, Enabled: true}
	}
	// Union with disabled-but-registered components so they show as "disabled"
	infraEntries := make([]InfraEntry, 0, len(infraByName))
	for _, name := range reg.RegisteredNames() {
		if e, ok := infraByName[name]; ok {
			infraEntries = append(infraEntries, e)
		} else {
			infraEntries = append(infraEntries, InfraEntry{Name: name, Enabled: false})
		}
	}
	// Sort: connected first, then enabled-but-disconnected, then disabled,
	// each group alphabetically.
	sort.Slice(infraEntries, func(i, j int) bool {
		a, b := infraEntries[i], infraEntries[j]
		rankA := 0
		if a.Connected {
			rankA = 2
		} else if a.Enabled {
			rankA = 1
		}
		rankB := 0
		if b.Connected {
			rankB = 2
		} else if b.Enabled {
			rankB = 1
		}
		if rankA != rankB {
			return rankA > rankB
		}
		return a.Name < b.Name
	})
	m.infraEntries = infraEntries

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
		switch registry.GetServiceState(name) {
		case "running":
			serviceEntries = append(serviceEntries, ServiceEntry{Name: name, Running: true})
		case "failed":
			serviceEntries = append(serviceEntries, ServiceEntry{Name: name, Failed: true})
		case "disabled":
			serviceEntries = append(serviceEntries, ServiceEntry{Name: name, Disabled: true})
		default:
			serviceEntries = append(serviceEntries, ServiceEntry{Name: name})
		}
	}
	// Sort: running services first, then stopped, each group alphabetically.
	sort.Slice(serviceEntries, func(i, j int) bool {
		a, b := serviceEntries[i], serviceEntries[j]
		if a.Running != b.Running {
			return a.Running
		}
		return a.Name < b.Name
	})
	m.serviceEntries = serviceEntries
}

func (m *TerminalModel) termProgressBar(percent float64, width int) string {
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
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled
	bar := m.percentStyle(percent).Render(strings.Repeat("█", filled)) +
		sidebarDimStyle().Render(strings.Repeat("░", empty))
	return bar
}

// ponytail: pastel progress stops at 3 thresholds — add more if smoother gradient needed
func (m *TerminalModel) percentStyle(percent float64) lipgloss.Style {
	switch {
	case percent < 50:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("success")))
	case percent < 80:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("warning")))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(TC("error")))
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
	t.program = tea.NewProgram(t.model, tea.WithAltScreen(), tea.WithMouseCellMotion())
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
