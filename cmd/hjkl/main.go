// Command hjkl launches the TUI for a multi-round navigation lesson.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/tui"
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	gen := challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), nil, solver.DefaultMaxDepth)

	lesson, err := curriculum.NewLesson(5, gen, challenge.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lesson: %v\n", err)
		os.Exit(1)
	}

	m := tui.NewLesson(lesson)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}