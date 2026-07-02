// Package tui renders the game state in the terminal and forwards
// keystrokes to the session. It is the only package that imports
// Bubble Tea / Charm libraries (ADR 0007).
package tui

import (
	"strings"

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

	keycastStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888"))

	pendingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ff8"))

	normalStyle = lipgloss.NewStyle()
)

const maxKeystrokes = 20

// Model is the Bubble Tea model for the hjkl TUI.
type Model struct {
	session    *session.Session
	challenge   challenge.Challenge
	goalRow     int
	goalCol     int
	keystrokes  []string // recent keystrokes for the keycast strip
}

// New creates a new Model for the given Challenge.
func New(c challenge.Challenge) Model {
	goalRow, goalCol := resolveGoalPosition(c)
	return Model{
		session:    session.New(c),
		challenge:   c,
		goalRow:     goalRow,
		goalCol:     goalCol,
		keystrokes:  make([]string, 0, maxKeystrokes),
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
				// Record keystroke for keycast strip.
				m.pushKeystroke(msg.String())
				m.session.Step(msg.String())
			}
			return m, nil
		}
	default:
		return m, nil
	}
}

// pushKeystroke appends a keystroke to the history, keeping at most maxKeystrokes.
func (m *Model) pushKeystroke(k string) {
	if len(m.keystrokes) >= maxKeystrokes {
		m.keystrokes = m.keystrokes[1:]
	}
	m.keystrokes = append(m.keystrokes, k)
}

// displayKey returns a display-friendly form of a keystroke.
func displayKey(k string) string {
	switch k {
	case " ":
		return "<space>"
	case "enter":
		return "<cr>"
	case "tab":
		return "<tab>"
	case "esc":
		return "<esc>"
	default:
		return k
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

	// Render each line of the buffer with the cursor and target highlighted.
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
		// If cursor is past the end of a line or on an empty line, show it.
		if row == cur.Row && (cur.Col >= len(line) || len(line) == 0) {
			pad := cur.Col - len(line)
			if pad < 0 {
				pad = 0
			}
			content += strings.Repeat(" ", pad) + cursorStyle.Render(" ")
		}
		content += "\n"
	}

	// --- Keycast strip ---
	var keycastLine string
	if len(m.keystrokes) > 0 {
		var b strings.Builder
		for _, k := range m.keystrokes {
			b.WriteString(displayKey(k))
		}
		keycastLine = keycastStyle.Render(b.String())
	}

	// Pending command indicator
	var pendingLine string
	if state.Pending != "" {
		pendingLine = pendingStyle.Render("Pending: "+displayKey(state.Pending)) + " "
	}

	// Goal description
	var goalLine string
	if !m.session.Solved() {
		goalLine = normalStyle.Render("Move cursor to the yellow target.")
	}

	// Combine everything
	var parts []string
	if solvedLine != "" {
		parts = append(parts, solvedLine)
	}
	parts = append(parts, content)
	if pendingLine != "" || keycastLine != "" {
		parts = append(parts, "")
		if pendingLine != "" {
			parts = append(parts, pendingLine)
		}
		if keycastLine != "" {
			parts = append(parts, keycastLine)
		}
	}
	if m.session.Solved() {
		parts = append(parts, "", normalStyle.Render("Press q to quit."))
	} else {
		parts = append(parts, "", goalLine)
	}

	return strings.Join(parts, "\n")
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
