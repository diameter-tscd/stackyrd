package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

func TestSidebarBlack(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	model := NewTerminalModel(LiveConfig{
		AppName: "stackyrd", AppVersion: "1.0", Port: "8080", Env: "dev",
		Banner: "  ___ _     \n / __| |_   \n \\__ \\  _|_ \n |___/\\__(_)\n            ",
	})
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	model.infraEntries = []InfraEntry{{Name: "redis", Connected: true, Enabled: true}, {Name: "mongo", Connected: false, Enabled: true}}
	model.serviceEntries = []ServiceEntry{{Name: "users_service", Running: true}, {Name: "tasks_service", Running: false}}

	view := model.View()
	lines := strings.Split(view, "\n")
	fmt.Printf("render lines=%d\n", len(lines))

	joined := stripANSI(view)
	for _, want := range []string{"Resources", "Components", "Services", "redis", "users_service", "RUNNING"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q", want)
		}
	}

	// The command block background must be #37353E (rendered as 48;2;55;52;62 due to termenv rounding)
	if !strings.Contains(view, "48;2;55;52;62") {
		t.Errorf("command block background #37353E not found in rendered output")
	}

	fmt.Println("BLACK SIDEBAR TEST OK")
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, c := range s {
		if c == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if c == 'm' || c == 'K' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}
