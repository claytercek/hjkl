// Command hjkl launches the TUI for a hardcoded multi-line challenge.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/tui"
	"github.com/clay/hjkl/internal/vim"
)

func main() {
	// Hardcoded multi-line challenge: navigate to a target position.
	c := challenge.New(
		vim.Buffer{Lines: []string{
			"hello world",
			"foo bar baz",
			"lorem ipsum",
		}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(2, 6), // the 'i' in "ipsum"
	)

	p := tea.NewProgram(tui.New(c), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
