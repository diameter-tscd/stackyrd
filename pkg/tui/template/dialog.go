package template

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DialogType represents the type of dialog
type DialogType int

const (
	DialogTypeConfirmation DialogType = iota
	DialogTypeInput
)

// DialogConfig contains configuration for a dialog
type DialogConfig struct {
	Type         DialogType
	Title        string
	Content      string
	InputPrompt  string
	DefaultValue string
	Width        int
}

// DialogResult represents the result of a dialog interaction
type DialogResult struct {
	Confirmed bool
	Value     string
	Cancelled bool
}

// DialogModel represents a reusable dialog component
type DialogModel struct {
	config      DialogConfig
	textinput   textinput.Model
	result      *DialogResult
	isActive    bool
	initialized bool
}

// NewDialog creates a new dialog with the given configuration
func NewDialog(config DialogConfig) *DialogModel {
	if config.Width == 0 {
		config.Width = 42
	}

	model := &DialogModel{
		config:   config,
		isActive: false, // Dialogs should not be active by default
	}

	if config.Type == DialogTypeInput {
		ti := textinput.New()
		ti.Placeholder = config.InputPrompt
		if config.InputPrompt == "" {
			ti.Placeholder = "Enter value..."
		}
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8daea5"))
		ti.CharLimit = 100
		ti.Width = config.Width - 4 // Account for padding
		ti.SetValue(config.DefaultValue)
		ti.Focus()
		model.textinput = ti
	}

	return model
}

// Show shows the dialog
func (d *DialogModel) Show() {
	d.isActive = true
	d.result = nil
}

// Hide hides the dialog
func (d *DialogModel) Hide() {
	d.isActive = false
}

// IsActive returns whether the dialog is currently active
func (d *DialogModel) IsActive() bool {
	return d.isActive
}

// GetResult returns the dialog result if completed, nil otherwise
func (d *DialogModel) GetResult() *DialogResult {
	return d.result
}

// Update handles dialog updates
func (d *DialogModel) Update(msg tea.Msg) tea.Cmd {
	if !d.isActive {
		return nil
	}

	switch d.config.Type {
	case DialogTypeConfirmation:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				d.result = &DialogResult{Confirmed: true}
				d.isActive = false
				return nil
			case "n", "N", "esc":
				d.result = &DialogResult{Confirmed: false, Cancelled: true}
				d.isActive = false
				return nil
			}
		}
	case DialogTypeInput:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				d.result = &DialogResult{
					Confirmed: true,
					Value:     d.textinput.Value(),
				}
				d.isActive = false
				return nil
			case "esc":
				d.result = &DialogResult{Cancelled: true}
				d.isActive = false
				return nil
			default:
				var cmd tea.Cmd
				d.textinput, cmd = d.textinput.Update(msg)
				return cmd
			}
		}
	}

	return nil
}

// View renders the dialog
func (d *DialogModel) View(width, height int) string {
	if !d.isActive {
		return ""
	}

	// Create full-screen black background
	blackLines := make([]string, height)
	for i := range blackLines {
		blackLines[i] = strings.Repeat(" ", width)
	}

	// Create dialog content
	var dialogContent strings.Builder

	// Title
	if d.config.Title != "" {
		dialogContent.WriteString(d.config.Title)
		dialogContent.WriteString("\n\n")
	}

	// Content/Input field
	switch d.config.Type {
	case DialogTypeConfirmation:
		if d.config.Content != "" {
			lines := strings.Split(d.config.Content, "\n")
			for _, line := range lines {
				dialogContent.WriteString(line)
				dialogContent.WriteString("\n")
			}
		}
	case DialogTypeInput:
		// Input field
		inputField := d.textinput.View()
		dialogContent.WriteString(inputField)
		dialogContent.WriteString("\n\n")

		// Instructions
		if d.config.Content != "" {
			dialogContent.WriteString(d.config.Content)
		} else {
			dialogContent.WriteString("Enter: confirm ● Esc: cancel")
		}
	}

	// Split content into lines
	contentLines := strings.Split(strings.TrimSuffix(dialogContent.String(), "\n"), "\n")
	maxContentWidth := d.config.Width

	// Center each line
	for i, line := range contentLines {
		if line != "" {
			// Center the line
			padding := (maxContentWidth - len(line)) / 2
			if padding > 0 {
				line = strings.Repeat(" ", padding) + line
			}
		}
		contentLines[i] = line
	}

	// Center the entire dialog block
	dialogHeight := len(contentLines)
	startY := (height - dialogHeight) / 2
	if startY < 0 {
		startY = 0
	}

	startX := (width - maxContentWidth) / 2
	if startX < 0 {
		startX = 0
	}

	// Place content lines on black background
	for i, contentLine := range contentLines {
		y := startY + i
		if y >= 0 && y < height && contentLine != "" {
			x := startX
			if x >= 0 && x < width {
				// Replace the black spaces with dialog content
				line := blackLines[y]
				contentToPlace := contentLine
				if len(contentToPlace) > width-x {
					contentToPlace = contentToPlace[:width-x]
				}
				if x+len(contentToPlace) <= len(line) {
					blackLines[y] = line[:x] + contentToPlace + line[x+len(contentToPlace):]
				} else if x < len(line) {
					visibleLen := len(line) - x
					if visibleLen > 0 {
						blackLines[y] = line[:x] + contentToPlace[:visibleLen]
					}
				}
			}
		}
	}

	return strings.Join(blackLines, "\n")
}

// Helper functions for common dialogs

// NewConfirmationDialog creates a confirmation dialog
func NewConfirmationDialog(title, message string) *DialogModel {
	return NewDialog(DialogConfig{
		Type:    DialogTypeConfirmation,
		Title:   title,
		Content: message,
	})
}

// NewInputDialog creates an input dialog
func NewInputDialog(title, prompt, defaultValue string) *DialogModel {
	return NewDialog(DialogConfig{
		Type:         DialogTypeInput,
		Title:        title,
		InputPrompt:  prompt,
		DefaultValue: defaultValue,
		Content:      "Enter: confirm ● Esc: cancel",
	})
}

// NewExitConfirmationDialog creates the standard exit confirmation dialog
func NewExitConfirmationDialog() *DialogModel {
	return NewConfirmationDialog("Exit Application", "Are you sure you want to exit? (y/N)")
}

// NewFilterDialog creates a filter input dialog
func NewFilterDialog(defaultValue string) *DialogModel {
	return NewInputDialog("Filter Logs", "Filter logs...", defaultValue)
}

// NewQueryDialog creates a query input dialog
func NewQueryDialog(defaultValue string) *DialogModel {
	return NewInputDialog("Command Query", "Enter Command...", defaultValue)
}
