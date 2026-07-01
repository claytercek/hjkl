// Command hjkl launches the TUI for a hardcoded single-line challenge.
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
	// Hardcoded single-line challenge: move cursor from column 0 to
	// the target position.
	c := challenge.New(
		vim.Buffer{Lines: []string{"hello world"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 7), // the 'o' in "world"
	)

	p := tea.NewProgram(tui.New(c), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
