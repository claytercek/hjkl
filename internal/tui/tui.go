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
	session   *session.Session
	challenge  challenge.Challenge
	goalRow    int
	goalCol    int
}

// New creates a new Model for the given Challenge.
func New(c challenge.Challenge) Model {
	goalRow, goalCol := resolveGoalPosition(c)
	return Model{
		session:   session.New(c),
		challenge:  c,
		goalRow:    goalRow,
		goalCol:    goalCol,
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
	targetRow, targetCol := m.goalRow, m.goalCol

	for row, line := range buf.Lines {
		for col, ch := range line {
			cell := string(ch)
			atCursor := row == cur.Row && col == cur.Col
			atTarget := row == targetRow && col == targetCol

			switch {
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

// resolveGoalPosition finds the target cell that satisfies the goal
// predicate. For the walking skeleton the only goal type is
// cursor-at-target, so we walk the buffer once at startup to locate it.
func resolveGoalPosition(c challenge.Challenge) (int, int) {
	initState := vim.State{Buffer: c.InitialBuffer, Cursor: c.InitialCursor}
	if c.Goal(initState) {
		return initState.Cursor.Row, initState.Cursor.Col
	}
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
