// Command hjkl launches the TUI for a hardcoded multi-line challenge.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/solver"
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

	// Compute optimal par for the full motion vocabulary.
	vocabulary := []string{
		"h", "j", "k", "l",
		"0", "^", "$",
		"w", "b", "e",
		"W", "B", "E",
		"f", "t", "F", "T", ";",
		"g", "G",
		// Include ASCII letters for f/t/F/T targets.
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
		"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
		// Include period for punctuation-heavy buffers.
		".",
	}
	// Note: "g"+"g" is handled by BFS like any other two-step command:
	// the first "g" sets Pending="g", the second "g" resolves to gg.

	sv := solver.New(c)
	par := sv.Solve(vocabulary, solver.DefaultMaxDepth)
	if par < 0 {
		par = -1
	}

	p := tea.NewProgram(tui.New(c, par), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}