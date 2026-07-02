// Command hjkl launches the TUI for a continuous mastery-driven challenge
// stream.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/tui"
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	gen := challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), nil, solver.DefaultMaxDepth)

	// Create the store for persisting progress.
	st, err := store.NewFileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create store: %v\n", err)
		os.Exit(1)
	}

	m := tui.NewStream(gen, rng, st)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
