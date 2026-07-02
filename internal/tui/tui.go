// Package tui renders the game state in the terminal and forwards
// keystrokes to the session. It is the only package that imports
// Bubble Tea / Charm libraries (ADR 0007).
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
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

	starStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0"))

	parInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#aaa"))

	normalStyle = lipgloss.NewStyle()

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0ff"))

	roundNumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88f"))

	templateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#aaa"))

	summaryLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#ffa"))
)

var emptyStarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#444"))

const maxKeystrokes = 20

// ---------------------------------------------------------------------------
// GameModel — renders one challenge (round)
// ---------------------------------------------------------------------------

// GameModel is the Bubble Tea model for a single challenge round.
type GameModel struct {
	session    *session.Session
	challenge  challenge.Challenge
	goalRow    int
	goalCol    int
	keystrokes []string // recent keystrokes for the keycast strip
}

// NewGame creates a new GameModel for the given Challenge.
func NewGame(c challenge.Challenge) GameModel {
	goalRow, goalCol := resolveGoalPosition(c)
	return GameModel{
		session:    session.New(c, c.Par),
		challenge:  c,
		goalRow:    goalRow,
		goalCol:    goalCol,
		keystrokes: make([]string, 0, maxKeystrokes),
	}
}

// Init implements tea.Model.
func (m GameModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m GameModel) Update(msg tea.Msg) (GameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.session.Solved() {
			m.pushKeystroke(msg.String())
			m.session.Step(msg.String())
		}
		return m, nil
	default:
		return m, nil
	}
}

// pushKeystroke appends a keystroke to the history, keeping at most maxKeystrokes.
func (m *GameModel) pushKeystroke(k string) {
	if len(m.keystrokes) >= maxKeystrokes {
		m.keystrokes = m.keystrokes[1:]
	}
	m.keystrokes = append(m.keystrokes, k)
}

// Solved returns true when the session is complete.
func (m GameModel) Solved() bool {
	return m.session.Solved()
}

// Result returns the session result.
func (m GameModel) Result() session.Result {
	return m.session.Result()
}

// ViewGame renders the challenge view.
func (m GameModel) ViewGame() string {
	state := m.session.State()
	buf := state.Buffer
	cur := state.Cursor

	// --- Solved banner ---
	var solvedLine string
	if m.session.Solved() {
		solvedLine = solvedStyle.Render("Solved!") + "\n"
		r := m.session.Result()
		if r.Par >= 0 {
			solvedLine += parInfoStyle.Render(
				fmt.Sprintf("you %d — par %d", r.Keystrokes, r.Par),
			) + "\n"
		} else {
			solvedLine += parInfoStyle.Render(
				fmt.Sprintf("you %d", r.Keystrokes),
			) + "\n"
		}
		solvedLine += starLine(r) + "\n\n"
	}

	// Render each line of the buffer.
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

	return strings.Join(parts, "\n")
}

// ---------------------------------------------------------------------------
// LessonModel — orchestrates a sequence of rounds and shows a summary
// ---------------------------------------------------------------------------

// lessonState tracks where we are in the lesson flow.
type lessonState int

const (
	statePlaying  lessonState = iota // a round is in progress
	stateRoundDone                   // current round solved, awaiting advance
	stateSummary                     // all rounds done, showing summary
)

// LessonModel is the Bubble Tea model for a multi-round lesson.
type LessonModel struct {
	lesson  *curriculum.Lesson
	current int      // 0-based index of current round
	game    GameModel // current round's game model
	state   lessonState
}

// NewLesson creates a LessonModel from a curriculum.Lesson.
func NewLesson(lesson *curriculum.Lesson) LessonModel {
	game := NewGame(lesson.Rounds[0].Challenge)
	return LessonModel{
		lesson:  lesson,
		current: 0,
		game:    game,
		state:   statePlaying,
	}
}

// Init implements tea.Model.
func (m LessonModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m LessonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ", "enter":
			if m.state == stateRoundDone {
				m.advanceToNextRound()
				return m, nil
			}
		}

		// Only forward keystrokes if a round is in progress.
		if m.state == statePlaying {
			var cmd tea.Cmd
			m.game, cmd = m.game.Update(msg)

			// Check if the round just completed.
			if m.game.Solved() {
				// Record the result on the lesson round.
				r := m.game.Result()
				m.lesson.Rounds[m.current].Result = curriculum.Result{
					Keystrokes: r.Keystrokes,
					Par:        r.Par,
					Stars:      r.Stars,
				}
				if m.current >= len(m.lesson.Rounds)-1 {
					m.state = stateSummary
				} else {
					m.state = stateRoundDone
				}
			}
			return m, cmd
		}
	}

	return m, nil
}

// advanceToNextRound creates the game model for the next round.
func (m *LessonModel) advanceToNextRound() {
	m.current++
	m.game = NewGame(m.lesson.Rounds[m.current].Challenge)
	m.state = statePlaying
}

// View implements tea.Model.
func (m LessonModel) View() string {
	switch m.state {
	case statePlaying:
		return m.viewPlaying()
	case stateRoundDone:
		return m.viewRoundDone()
	case stateSummary:
		return m.viewSummary()
	default:
		return normalStyle.Render("unknown state")
	}
}

// viewPlaying renders the current challenge.
func (m LessonModel) viewPlaying() string {
	// Header: round progress
	header := headerStyle.Render(fmt.Sprintf("Round %d / %d", m.current+1, len(m.lesson.Rounds)))

	// Template info
	tmpl := m.lesson.Rounds[m.current].Template
	tmplLine := templateStyle.Render(tmpl.String())

	gameView := m.game.ViewGame()

	// Goal description
	var goalLine string
	if !m.game.Solved() {
		goalLine = normalStyle.Render("Move cursor to the yellow target.")
	}

	parts := []string{
		header,
		tmplLine,
		"",
		gameView,
		"",
		goalLine,
	}
	return strings.Join(parts, "\n")
}

// viewRoundDone shows the solved state and prompt to continue.
func (m LessonModel) viewRoundDone() string {
	header := headerStyle.Render(fmt.Sprintf("Round %d / %d", m.current+1, len(m.lesson.Rounds)))
	gameView := m.game.ViewGame()

	remaining := len(m.lesson.Rounds) - m.current - 1
	var nextLine string
	if remaining == 1 {
		nextLine = normalStyle.Render("1 round remaining. Press space for next round.")
	} else {
		nextLine = normalStyle.Render(fmt.Sprintf("%d rounds remaining. Press space for next round.", remaining))
	}

	parts := []string{
		header,
		"",
		gameView,
		"",
		nextLine,
	}
	return strings.Join(parts, "\n")
}

// viewSummary renders the lesson summary screen.
func (m LessonModel) viewSummary() string {
	summary := m.lesson.ComputeSummary()

	var b strings.Builder
	b.WriteString(headerStyle.Render("Lesson Complete!") + "\n\n")

	for i, r := range summary.Rounds {
		tmplName := r.Template.String()
		roundLabel := roundNumStyle.Render(fmt.Sprintf("Round %d", i+1))
		tmplLabel := templateStyle.Render(tmplName)

		var resultLine string
		if r.Result.Keystrokes > 0 || r.Result.Stars > 0 {
			if r.Result.Par >= 0 {
				resultLine = fmt.Sprintf("  %s  %s\n  you %d — par %d  %s",
					roundLabel, tmplLabel, r.Result.Keystrokes, r.Result.Par,
					starLineShort(r.Result.Stars))
			} else {
				resultLine = fmt.Sprintf("  %s  %s\n  you %d  %s",
					roundLabel, tmplLabel, r.Result.Keystrokes,
					starLineShort(r.Result.Stars))
			}
		} else {
			resultLine = fmt.Sprintf("  %s  %s  — not played", roundLabel, tmplLabel)
		}
		b.WriteString(resultLine + "\n\n")
	}

	// Aggregate.
	b.WriteString(summaryLabelStyle.Render("Total") + "\n")
	totalLine := fmt.Sprintf("  keystrokes: %d  par: %d  stars: %d  %s",
		summary.TotalKeystrokes, summary.TotalPar, summary.TotalStars,
		summaryStars(summary.TotalStars))
	b.WriteString(totalLine + "\n\n")
	b.WriteString(normalStyle.Render("Press q to quit."))

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// starLine renders the star band for a result.
func starLine(r session.Result) string {
	var b strings.Builder
	for i := 1; i <= 3; i++ {
		if i <= r.Stars {
			b.WriteString(starStyle.Render("★"))
		} else {
			b.WriteString(emptyStarStyle.Render("☆"))
		}
	}
	return b.String()
}

// starLineShort renders a compact star band (no empty stars).
func starLineShort(stars int) string {
	var b strings.Builder
	for i := 1; i <= stars; i++ {
		b.WriteString(starStyle.Render("★"))
	}
	return b.String()
}

// summaryStars renders stars for the total.
func summaryStars(total int) string {
	return starStyle.Render(strings.Repeat("★", total))
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