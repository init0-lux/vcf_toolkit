package tui

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Run starts the TUI program. stdin/stdout/stderr allow embedding in tests or
// alternate shells.
func Run(stdin io.Reader, stdout, stderr io.Writer) error {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	m := newModel(stderr)
	p := tea.NewProgram(
		m,
		tea.WithInput(stdin),
		tea.WithOutput(stdout),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

var (
	colorTitle  = lipgloss.AdaptiveColor{Light: "#2b2b2b", Dark: "#f2f2f2"}
	colorDim    = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#a3a3a3"}
	colorAccent = lipgloss.AdaptiveColor{Light: "#005f87", Dark: "#7dd3fc"}
)

func banner(width int) string {
	title := lipgloss.NewStyle().
		Foreground(colorTitle).
		Bold(true).
		Render("vcf-toolkit")

	sub := lipgloss.NewStyle().
		Foreground(colorDim).
		Render("Contact cleanup toolkit (normalize / dedupe / convert)")

	block := lipgloss.JoinVertical(lipgloss.Left, title, sub)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, block)
}

func warn(err error) string {
	if err == nil {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("Error: %v", err))
}
