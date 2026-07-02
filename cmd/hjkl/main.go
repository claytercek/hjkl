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
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/tui"
	"github.com/clay/hjkl/internal/vim"
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	gen := challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), nil, solver.DefaultMaxDepth)

	// Demo: first round uses hand-placed walls to showcase the mechanic.
	// "hello world", cursor at col 0, walls at cols 1-4 block "ello ",
	// target at col 6 (start of "world"). Press 'w' to jump over walls.
	demoWall := challenge.NewWithWalls(
		vim.Buffer{Lines: []string{"hello world"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
		challenge.WallSet{
			vim.Cursor{Row: 0, Col: 1}: true,
			vim.Cursor{Row: 0, Col: 2}: true,
			vim.Cursor{Row: 0, Col: 3}: true,
			vim.Cursor{Row: 0, Col: 4}: true,
		},
	)
	demoWall.Par = 1 // 'w' is the optimal solution

	// Generate the remaining rounds.
	lesson, err := curriculum.NewLesson(5, gen, challenge.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lesson: %v\n", err)
		os.Exit(1)
	}

	// Prepend the demo round.
	demoRound := curriculum.Round{
		Challenge: demoWall,
		Template:  challenge.THorizontalLine,
	}
	lesson.Rounds = append([]curriculum.Round{demoRound}, lesson.Rounds...)

	// Create the store for persisting progress.
	st, err := store.NewFileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create store: %v\n", err)
		os.Exit(1)
	}

	// Load persisted progress to determine how many groups are unlocked.
	progress, _ := st.LoadProgress()
	unlockedCount := progress.UnlockedCount
	if unlockedCount < 1 {
		unlockedCount = 1
	}

	// Build the challenge generator (full vocabulary for internal par checks).
	gen := challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)

	// Create the learning stream.
	config := challenge.DefaultConfig()
	stream := curriculum.NewStream(gen, config, rng, progress, unlockedCount)

	m := tui.NewLessonStream(stream, st)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}