// Package tui renders the game state in the terminal and forwards
// keystrokes to the session. It is the only package that imports
// Bubble Tea / Charm libraries (ADR 0007).
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/session"
	"github.com/clay/hjkl/internal/vim"
)

// Styles
var (
	targetStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#ff0")).
			Foreground(lipgloss.Color("#000"))

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#44f")).
			Foreground(lipgloss.Color("#fff"))

	solvedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0f0"))

	normalStyle = lipgloss.NewStyle()
)

// Model is the Bubble Tea model for the hjkl TUI.
type Model struct {
	session  *session.Session
	challenge challenge.Challenge
}

// New creates a new Model for the given Challenge.
func New(c challenge.Challenge) Model {
	return Model{
		session:  session.New(c),
		challenge: c,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			if !m.session.Solved() {
				m.session.Step(msg.String())
			}
			return m, nil
		}
	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m Model) View() string {
	state := m.session.State()
	buf := state.Buffer
	cur := state.Cursor

	var solvedLine string
	if m.session.Solved() {
		solvedLine = solvedStyle.Render("Solved!") + "\n\n"
	}

	// Render each line of the buffer with the cursor highlighted.
	var content string
	targetRow, targetCol := targetPosition(m.challenge)

	for row, line := range buf.Lines {
		for col, ch := range line {
			cell := string(ch)
			atCursor := row == cur.Row && col == cur.Col
			atTarget := row == targetRow && col == targetCol

			switch {
			case atCursor && atTarget:
				// Highlight cursor overrides when both at same spot
				content += cursorStyle.Render(cell)
			case atCursor:
				content += cursorStyle.Render(cell)
			case atTarget:
				content += targetStyle.Render(cell)
			default:
				content += normalStyle.Render(cell)
			}
		}
		// If cursor is past the end of an empty line, show it.
		if row == cur.Row && len(line) == 0 {
			content += cursorStyle.Render(" ")
		}
		content += "\n"
	}

	// Keystroke prompt
	var prompt string
	if m.session.Solved() {
		prompt = "Press q to quit."
	} else {
		prompt = "Use h and l to move the cursor to the yellow target."
	}

	return fmt.Sprintf("%s%s\n%s", solvedLine, content, prompt)
}

// targetPosition extracts the target row/col from the challenge's goal
// predicate by building the initial state and checking it.
// This is a bit hacky for now — in a fuller implementation the Challenge
// would carry target metadata directly.
func targetPosition(c challenge.Challenge) (int, int) {
	// We scan from the initial state forward; for the walking skeleton
	// we know the target is just the cursor-at-target predicate.
	// Since we can't inspect the closure, we brute-force by trying
	// each cell to find which one satisfies the goal.
	initState := vim.State{Buffer: c.InitialBuffer, Cursor: c.InitialCursor}
	// Check initial position first (unlikely but possible).
	if c.Goal(initState) {
		return initState.Cursor.Row, initState.Cursor.Col
	}
	// Walk all positions to find the target.
	for row := range initState.Buffer.Lines {
		for col := range initState.Buffer.Lines[row] {
			s := vim.State{Buffer: initState.Buffer, Cursor: vim.Cursor{Row: row, Col: col}}
			if c.Goal(s) {
				return row, col
			}
		}
	}
	return 0, 0
}
