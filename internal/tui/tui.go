// Package tui renders the game state in the terminal and forwards
// keystrokes to the session. It is the only package that imports
// Bubble Tea / Charm libraries (ADR 0007).
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/session"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/vim"
)

// ---------------------------------------------------------------------------
// Configuration / key bindings
// ---------------------------------------------------------------------------

// KeyBindings defines the relocatable key bindings used by the TUI.
// All are config-driven with sensible hardcoded defaults.
type KeyBindings struct {
	Pause string // toggle pause overlay (default "ctrl+c")
	Hint  string // reveal next optimal keystroke (default "ctrl+h")
	Skip  string // skip current round (default "ctrl+n")
	Retry string // retry current challenge (default "ctrl+r")
}

// DefaultBindings returns the default key bindings.
func DefaultBindings() KeyBindings {
	return KeyBindings{
		Pause: "ctrl+c",
		Hint:  "ctrl+h",
		Skip:  "ctrl+n",
		Retry: "ctrl+r",
	}
}

// Config holds all TUI configuration.
type Config struct {
	Bindings      KeyBindings
	ReducedMotion bool // when true all animation is instant
}

// DefaultConfig returns the default TUI configuration.
func DefaultConfig() Config {
	return Config{
		Bindings:      DefaultBindings(),
		ReducedMotion: false,
	}
}

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	targetStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#ff0")).
			Foreground(lipgloss.Color("#000"))

	wallStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#622")).
			Foreground(lipgloss.Color("#a55"))

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#44f")).
			Foreground(lipgloss.Color("#fff"))

	ghostStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#225")).
			Foreground(lipgloss.Color("#889"))

	ghostFaintStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#113")).
			Foreground(lipgloss.Color("#667"))

	solvedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0f0"))

	solvedBurstStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("#0f0")).
				Foreground(lipgloss.Color("#000"))

	keycastStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888"))

	pendingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ff8"))

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

	// Wasted key shake
	wasteBackground = lipgloss.NewStyle().
			Background(lipgloss.Color("#800")).
			Foreground(lipgloss.Color("#fcc"))

	hintStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#282")).
			Foreground(lipgloss.Color("#0f0"))

	// Pause overlay
	menuTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0ff"))

	menuItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ccc"))

	menuSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("#448")).
				Foreground(lipgloss.Color("#fff"))

	// Footer
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666"))

	// Unlock progress bar
	unlockBarFilledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#556"))
	unlockBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#223"))
)

const maxKeystrokes = 20

// ---------------------------------------------------------------------------
// Animation types
// ---------------------------------------------------------------------------

const (
	tickInterval  = 50 * time.Millisecond
	ghostMaxLife  = 10   // frames before a ghost fades
	shakeFrames   = 6    // frames of shake animation on wasted key
	successFrames = 15   // frames of success burst animation
)

type tickMsg time.Time

// ghostPos tracks a single ghost trail cell.
type ghostPos struct {
	row, col int
	life     int // remaining animation frames
}

// ---------------------------------------------------------------------------
// GameModel — renders one challenge (round)
// ---------------------------------------------------------------------------

// GameModel is the Bubble Tea model for a single challenge round.
type GameModel struct {
	session   *session.Session
	challenge challenge.Challenge
	goalRow   int
	goalCol   int
	keystrokes []string

	// Ghost trail
	prevCursor vim.Cursor
	ghosts     []ghostPos

	// Cache: optimal distance from previous step (avoids redundant solver call).
	// Initialised in NewGame.
	lastOptimal int

	// Effects
	wasteFrames   int   // non-zero when a wasted-key shake is active
	successFrames int   // non-zero when a success burst is playing
	hintKey       string // optimal next keystroke from solver, "" if none or unsolvable
	hintVisible   bool

	// Animation control
	reducedMotion bool

	// Binding
	bindings KeyBindings
}

// NewGame creates a new GameModel for the given Challenge.
func NewGame(c challenge.Challenge, bindings KeyBindings, reducedMotion bool) GameModel {
	goalRow, goalCol := resolveGoalPosition(c)
	// Pre-compute optimal distance from the initial state.
	initialState := vim.State{Buffer: c.InitialBuffer, Cursor: c.InitialCursor, DesiredCol: -1}
	lastOpt := solver.OptimalFromState(initialState, c, challenge.NavigationVocabulary, solver.DefaultMaxDepth)
	return GameModel{
		session:       session.New(c, c.Par),
		challenge:     c,
		goalRow:       goalRow,
		goalCol:       goalCol,
		keystrokes:    make([]string, 0, maxKeystrokes),
		bindings:      bindings,
		reducedMotion: reducedMotion,
		lastOptimal:   lastOpt,
	}
}

// Init implements tea.Model.
func (m GameModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. This processes keystrokes and forwards
// them to the session, then sets up animation state.
func (m GameModel) Update(msg tea.Msg) (GameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.session.Solved() {
			m.applyKeystroke(msg.String())
		}
		return m, m.createTickCmd()
	default:
		return m, nil
	}
}

// applyKeystroke applies one keystroke to the session and sets up
// animation state (ghost trail, wasted-key detection, success burst).
// Logical state updates atomically — all animation is a non-blocking
// overlay that is abandoned on the next keystroke.
func (m *GameModel) applyKeystroke(k string) {
	if m.session.Solved() {
		return
	}

	// Snapshot pre-keystroke state.
	prevState := m.session.State()
	m.prevCursor = prevState.Cursor

	// Abandon any in-flight animation.
	m.ghosts = nil
	m.wasteFrames = 0

	// Use the cached optimal distance from the previous step as the
	// pre-keystroke value (avoids a redundant solver call).
	preOpt := m.lastOptimal

	m.pushKeystroke(k)
	m.session.Step(k)

	newState := m.session.State()

	// Ghost trail from old to new cursor.
	// Skip when cursor didn't move (e.g. wall refusal, buffer edge).
	if !m.reducedMotion && (m.prevCursor.Row != newState.Cursor.Row || m.prevCursor.Col != newState.Cursor.Col) {
		m.addGhostTrail(m.prevCursor, newState.Cursor)
	}

	// Wasted-key detection: a key is wasted when it increases the
	// optimal remaining distance or keeps it unchanged while the
	// cursor stays put (no-op at buffer edge, pending command that
	// doesn't resolve, etc.). A key that keeps distance unchanged
	// but moves the cursor is suboptimal, not wasted.
	if !m.session.Solved() {
		postOpt := solver.OptimalFromState(newState, m.challenge, challenge.NavigationVocabulary, solver.DefaultMaxDepth)
		m.lastOptimal = postOpt
		cursorMoved := m.prevCursor.Row != newState.Cursor.Row || m.prevCursor.Col != newState.Cursor.Col
		wasted := preOpt >= 0 && (postOpt < 0 || postOpt > preOpt || (postOpt == preOpt && !cursorMoved))
		if wasted {
			m.wasteFrames = shakeFrames
		}
	} else {
		// Solved — set success burst.
		m.successFrames = successFrames
	}

	// Clear hint on any keystroke.
	m.hintKey = ""
	m.hintVisible = false
}

// advanceAnimations decrements animation frame counters and removes
// expired ghosts. Returns the number of active animations.
func (m *GameModel) advanceAnimations() int {
	// Advance ghosts.
	active := 0
	for i := range m.ghosts {
		m.ghosts[i].life--
		if m.ghosts[i].life > 0 {
			active++
		}
	}
	// Compact ghosts.
	if active < len(m.ghosts) {
		kept := m.ghosts[:0]
		for _, g := range m.ghosts {
			if g.life > 0 {
				kept = append(kept, g)
			}
		}
		m.ghosts = kept
	}

	// Advance waste frames.
	if m.wasteFrames > 0 {
		m.wasteFrames--
		active++
	}

	// Advance success frames.
	if m.successFrames > 0 {
		m.successFrames--
		active++
	}

	return active
}

// createTickCmd returns a tea.Tick command if there are active animations.
func (m *GameModel) createTickCmd() tea.Cmd {
	if m.wasteFrames > 0 || m.successFrames > 0 || len(m.ghosts) > 0 {
		return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return nil
}

// addGhostTrail creates ghost positions along the path from `from` to `to`.
// In reduced-motion mode this is a no-op.
func (m *GameModel) addGhostTrail(from, to vim.Cursor) {
	positions := interpolate(from, to, m.session.State().Buffer)
	for i, pos := range positions {
		// Give ghosts near the start a shorter life so the trail fades
		// from old toward new.
		life := ghostMaxLife - (len(positions)-1-i)*ghostMaxLife/len(positions)
		if life < 2 {
			life = 2
		}
		m.ghosts = append(m.ghosts, ghostPos{row: pos.Row, col: pos.Col, life: life})
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

// ghostAt returns the ghost with the longest remaining life at the given
// position, or 0 if none.
func (m GameModel) ghostAt(row, col int) int {
	for _, g := range m.ghosts {
		if g.row == row && g.col == col {
			return g.life
		}
	}
	return 0
}

// ViewGame renders the challenge view.
func (m GameModel) ViewGame() string {
	state := m.session.State()
	buf := state.Buffer
	cur := state.Cursor

	// --- Solved banner ---
	var solvedLine string
	if m.session.Solved() {
		if m.reducedMotion || m.successFrames <= 0 {
			solvedLine = solvedStyle.Render("Solved!") + "\n"
		} else {
			// Burst: pulse between normal and bright.
			burst := m.successFrames%4 < 2
			if burst {
				solvedLine = solvedBurstStyle.Render("Solved!") + "\n"
			} else {
				solvedLine = solvedStyle.Render("Solved!") + "\n"
			}
		}
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
	}

	// --- Hint display ---
	var hintLine string
	if m.hintVisible && m.hintKey != "" {
		hintLine = hintStyle.Render("Hint: "+displayKey(m.hintKey)) + "\n\n"
	}

	// Render each line of the buffer.
	var content string
	targetRow, targetCol := m.goalRow, m.goalCol
	shaking := m.wasteFrames > 0 && !m.reducedMotion

	for row, line := range buf.Lines {
		for col, ch := range line {
			cell := string(ch)
			atCursor := row == cur.Row && col == cur.Col
			atTarget := row == targetRow && col == targetCol
			ghostLife := m.ghostAt(row, col)

			switch {
			case shaking && atCursor:
				// Shake: render with waste style.
				content += wasteBackground.Render(cell)
			case m.challenge.IsWall(row, col):
				content += wallStyle.Render(cell)
			case atCursor:
				content += cursorStyle.Render(cell)
			case ghostLife > 0 && !m.reducedMotion:
				// Ghost: dimmer style based on remaining life.
				if ghostLife > ghostMaxLife/2 {
					content += ghostStyle.Render(cell)
				} else {
					content += ghostFaintStyle.Render(cell)
				}
			case atTarget:
				content += targetStyle.Render(cell)
			default:
				content += normalStyle.Render(cell)
			}
		}
		// Cursor past end of line.
		if row == cur.Row && (cur.Col >= len(line) || len(line) == 0) {
			pad := cur.Col - len(line)
			if pad < 0 {
				pad = 0
			}
			if shaking {
				content += strings.Repeat(" ", pad) + wasteBackground.Render(" ")
			} else {
				content += strings.Repeat(" ", pad) + cursorStyle.Render(" ")
			}
		}
		// Ghost past end of line.
		if !m.reducedMotion {
			for _, g := range m.ghosts {
				if g.row == row && g.col >= len(line) && g.life > 0 {
					pad := g.col - len(line)
					content += strings.Repeat(" ", pad)
					if g.life > ghostMaxLife/2 {
						content += ghostStyle.Render(" ")
					} else {
						content += ghostFaintStyle.Render(" ")
					}
				}
			}
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
	if hintLine != "" {
		parts = append(parts, hintLine)
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
// Pause overlay
// ---------------------------------------------------------------------------

var pauseMenuItems = []string{"Resume", "Skip", "Retry", "Restart Lesson", "Quit"}

// viewPauseOverlay renders the pause/menu overlay.
func viewPauseOverlay(selected int) string {
	var b strings.Builder
	b.WriteString(menuTitleStyle.Render("Menu") + "\n\n")

	for i, item := range pauseMenuItems {
		if i == selected {
			b.WriteString(menuSelectedStyle.Render(" > "+item) + "\n")
		} else {
			b.WriteString(menuItemStyle.Render("   "+item) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("j/k or arrows to navigate, Enter to select"))

	return b.String()
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
	lesson     *curriculum.Lesson
	current    int // 0-based index of current round
	game       GameModel
	state      lessonState

	// Pause overlay
	paused     bool
	menuCursor int

	// Configuration
	config Config

	// Hint state
	hintActive bool

	// Progress persistence (may be nil)
	store    store.Store
	progress store.Progress // in-memory copy of progress (updated live)
}

// NewLesson creates a LessonModel from a curriculum.Lesson.
// If a store is provided, persisted progress is loaded.
func NewLesson(lesson *curriculum.Lesson, s ...store.Store) LessonModel {
	var st store.Store
	if len(s) > 0 {
		st = s[0]
	}
	return newLesson(lesson, DefaultConfig(), st)
}

// NewLessonWithConfig creates a LessonModel with the given config.
func NewLessonWithConfig(lesson *curriculum.Lesson, cfg Config) LessonModel {
	return newLesson(lesson, cfg, nil)
}

// newLesson is the shared constructor.
func newLesson(lesson *curriculum.Lesson, cfg Config, s store.Store) LessonModel {
	game := NewGame(lesson.Rounds[0].Challenge, cfg.Bindings, cfg.ReducedMotion)
	var p store.Progress
	if s != nil {
		p, _ = s.LoadProgress()
	}
	// Ensure maps are never nil so updateProgress can write to them.
	if p.BestScores == nil {
		p.BestScores = make(map[store.GroupKey]store.BestScore)
	}
	if p.Mastery == nil {
		p.Mastery = make(map[store.GroupKey]store.Mastery)
	}
	return LessonModel{
		lesson:   lesson,
		current:  0,
		game:     game,
		state:    statePlaying,
		config:   cfg,
		store:    s,
		progress: p,
	}
}

// Init implements tea.Model.
func (m LessonModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m LessonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Advance animations during any non-paused state.
		if m.paused || m.state == stateSummary {
			return m, nil
		}
		m.game.advanceAnimations()
		// If there are still active animations, keep ticking.
		active := m.game.wasteFrames > 0 || m.game.successFrames > 0 || len(m.game.ghosts) > 0
		if active {
			return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg {
				return tickMsg(t)
			})
		}
		return m, nil

	case tea.KeyMsg:
		// Handle pause toggle globally. Esc is never an app control,
		// so Ctrl-C toggles the menu. Pause only works during active
		// play states (playing, roundDone) and when already paused.
		if msg.String() == m.config.Bindings.Pause {
			if m.paused || m.state == statePlaying || m.state == stateRoundDone {
				m.paused = !m.paused
				m.menuCursor = 0
				return m, nil
			}
			// Can't pause in summary.
			return m, nil
		}

		// Handle menu navigation when paused.
		if m.paused {
			cmd := m.handleMenuKey(msg)
			return m, cmd
		}

		// Esc is never intercepted by the app (reserved for vim).
		if msg.String() == "esc" {
			// Pass through to game.
			if m.state == statePlaying {
				var cmd tea.Cmd
				m.game, cmd = m.game.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch {
		case m.state == stateSummary:
			if msg.String() == "q" {
				m.saveProgress()
				return m, tea.Quit
			}
			return m, nil

		case msg.String() == m.config.Bindings.Hint:
			if m.state == statePlaying && !m.game.Solved() {
				m.showHint()
			}
			return m, nil

		case msg.String() == m.config.Bindings.Retry:
			if m.state == statePlaying || m.state == stateRoundDone {
				m.retryRound()
				return m, nil
			}

		case msg.String() == m.config.Bindings.Skip:
			m.skipRound()
			return m, nil

		case msg.String() == " ", msg.String() == "enter":
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
				r := m.game.Result()

				// Record the result on the lesson round.
				m.lesson.Rounds[m.current].Result = curriculum.Result{
					Keystrokes: r.Keystrokes,
					Par:        r.Par,
					Stars:      r.Stars,
				}

				// Update persisted progress (best score + mastery).
				groupKey := curriculum.GroupForTemplate(m.lesson.Rounds[m.current].Template)
				m.updateProgress(groupKey, r)

				if m.current >= len(m.lesson.Rounds)-1 {
					m.state = stateSummary
					// Save progress on lesson completion.
					m.saveProgress()
				} else {
					m.state = stateRoundDone
				}
			}
			return m, cmd
		}
	}

	return m, nil
}

// updateProgress updates the in-memory progress with the round result.
func (m *LessonModel) updateProgress(groupKey string, r session.Result) {
	key := store.GroupKey(groupKey)

	// Update best score.
	current := m.progress.BestScores[key]
	m.progress.BestScores[key] = store.UpdateBestScore(current, r.Keystrokes, r.Par, r.Stars)

	// Update mastery EWMA.
	prev := m.progress.Mastery[key]
	m.progress.Mastery[key] = store.UpdateMastery(prev, r.Keystrokes, r.Par, r.Stars, store.DefaultAlpha)
}

// saveProgress persists the current in-memory progress through the store.
func (m *LessonModel) saveProgress() {
	if m.store != nil {
		_ = m.store.SaveProgress(m.progress)
	}
}

// handleMenuKey processes key events while the pause overlay is active.
// It returns a tea.Cmd when a selection produces an action, or nil
// for navigation keys.
func (m *LessonModel) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		m.menuCursor = (m.menuCursor + 1) % len(pauseMenuItems)
		return nil
	case "k", "up":
		m.menuCursor = (m.menuCursor - 1 + len(pauseMenuItems)) % len(pauseMenuItems)
		return nil
	case "enter", " ":
		return m.executeMenuSelection()
	}
	return nil
}

// executeMenuSelection runs the action for the currently selected menu item
// and returns the resulting tea.Cmd (nil for most actions, tea.Quit for quit).
func (m *LessonModel) executeMenuSelection() tea.Cmd {
	item := pauseMenuItems[m.menuCursor]
	switch item {
	case "Resume":
		m.paused = false
		return nil
	case "Skip":
		m.paused = false
		m.hintActive = false
		m.skipRound()
		return nil
	case "Retry":
		m.paused = false
		m.hintActive = false
		m.retryRound()
		return nil
	case "Restart Lesson":
		m.paused = false
		m.hintActive = false
		m.restartLesson()
		return nil
	case "Quit":
		m.saveProgress()
		return tea.Quit
	}
	return nil
}

// showHint computes and displays the optimal next keystroke.
func (m *LessonModel) showHint() {
	state := m.game.session.State()
	vocab := challenge.NavigationVocabulary
	maxDepth := solver.DefaultMaxDepth
	key, _ := solver.FirstStepFromState(state, m.game.challenge, vocab, maxDepth)
	m.hintActive = true
	if key != "" {
		m.game.hintKey = key
		m.game.hintVisible = true
	}
}

// skipRound advances to the next round, marking the current one as skipped.
func (m *LessonModel) skipRound() {
	m.lesson.Rounds[m.current].Result = curriculum.Result{
		Keystrokes: 0,
		Par:        m.lesson.Rounds[m.current].Challenge.Par,
		Stars:      0,
	}
	if m.current >= len(m.lesson.Rounds)-1 {
		m.state = stateSummary
	} else {
		m.current++
		m.game = NewGame(m.lesson.Rounds[m.current].Challenge, m.config.Bindings, m.config.ReducedMotion)
		m.state = statePlaying
	}
}

// retryRound restarts the current round, clearing its result.
func (m *LessonModel) retryRound() {
	m.lesson.Rounds[m.current].Result = curriculum.Result{}
	m.game = NewGame(m.lesson.Rounds[m.current].Challenge, m.config.Bindings, m.config.ReducedMotion)
	m.state = statePlaying
}

// restartLesson goes back to the first round, clearing all round results.
func (m *LessonModel) restartLesson() {
	for i := range m.lesson.Rounds {
		m.lesson.Rounds[i].Result = curriculum.Result{}
	}
	m.current = 0
	m.game = NewGame(m.lesson.Rounds[0].Challenge, m.config.Bindings, m.config.ReducedMotion)
	m.state = statePlaying
}

// advanceToNextRound creates the game model for the next round.
func (m *LessonModel) advanceToNextRound() {
	m.current++
	m.game = NewGame(m.lesson.Rounds[m.current].Challenge, m.config.Bindings, m.config.ReducedMotion)
	m.state = statePlaying
	m.hintActive = false
}

// View implements tea.Model.
func (m LessonModel) View() string {
	// If paused, overlay the menu on top of the current content.
	if m.paused {
		return m.viewPaused()
	}

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

// viewPlaying renders the current challenge with footer.
func (m LessonModel) viewPlaying() string {
	header := headerStyle.Render(fmt.Sprintf("Round %d / %d", m.current+1, len(m.lesson.Rounds)))
	tmpl := m.lesson.Rounds[m.current].Template
	tmplLine := templateStyle.Render(tmpl.String())
	gameView := m.game.ViewGame()

	var goalLine string
	if !m.game.Solved() {
		goalLine = normalStyle.Render("Move cursor to the yellow target.")
	}

	unlockBar := m.renderUnlockProgress()
	footer := m.renderFooter()

	parts := []string{
		header,
		tmplLine,
		"",
		gameView,
		"",
		goalLine,
	}
	if unlockBar != "" {
		parts = append(parts, "", unlockBar)
	}
	parts = append(parts, "", footer)
	return strings.Join(parts, "\n")
}

// viewRoundDone shows the solved state and prompt to continue with footer.
func (m LessonModel) viewRoundDone() string {
	header := headerStyle.Render(fmt.Sprintf("Round %d / %d", m.current+1, len(m.lesson.Rounds)))
	gameView := m.game.ViewGame()

	retryLabel := fmt.Sprintf("%s: retry", displayKey(m.config.Bindings.Retry))
	advanceLabel := fmt.Sprintf("%s: next", displayKey(" "))
	nextLine := normalStyle.Render(strings.Join([]string{retryLabel, advanceLabel}, "  ·  "))

	footer := m.renderFooter()

	parts := []string{
		header,
		"",
		gameView,
		"",
		nextLine,
		"",
		footer,
	}
	return strings.Join(parts, "\n")
}

// viewPaused renders the pause overlay.
func (m LessonModel) viewPaused() string {
	// Show the current game content behind the overlay.
	var content string
	switch m.state {
	case statePlaying:
		content = m.viewPlaying()
	case stateRoundDone:
		content = m.viewRoundDone()
	default:
		content = m.viewPlaying()
	}

	overlay := viewPauseOverlay(m.menuCursor)

	// Combine: game content, then overlay separated by blank lines.
	return content + "\n\n" + overlay
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
		if r.Result.Keystrokes > 0 {
			if r.Result.Par >= 0 {
				resultLine = fmt.Sprintf("  %s  %s\n  you %d — par %d",
					roundLabel, tmplLabel, r.Result.Keystrokes, r.Result.Par)
			} else {
				resultLine = fmt.Sprintf("  %s  %s\n  you %d",
					roundLabel, tmplLabel, r.Result.Keystrokes)
			}
		} else {
			resultLine = fmt.Sprintf("  %s  %s  — not played", roundLabel, tmplLabel)
		}
		b.WriteString(resultLine + "\n\n")
	}

	// Aggregate.
	b.WriteString(summaryLabelStyle.Render("Total") + "\n")
	totalLine := fmt.Sprintf("  keystrokes: %d  par: %d",
		summary.TotalKeystrokes, summary.TotalPar)
	b.WriteString(totalLine + "\n\n")

	b.WriteString(normalStyle.Render("Press q to quit."))
	return b.String()
}

// viewBestProgress renders the historical bests and mastery section of the summary.
func (m LessonModel) viewBestProgress() string {
	if len(m.progress.BestScores) == 0 && len(m.progress.Mastery) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(summaryLabelStyle.Render("Progress") + "\n")

	// Collect all group keys present in progress.
	keys := make(map[store.GroupKey]bool)
	for k := range m.progress.BestScores {
		keys[k] = true
	}
	for k := range m.progress.Mastery {
		keys[k] = true
	}

	for key := range keys {
		groupLabel := templateStyle.Render(string(key))

		// Best score line.
		if bs, ok := m.progress.BestScores[key]; ok && bs.Stars > 0 {
			bsLine := fmt.Sprintf("  %s  best: you %d — par %d  %s",
				groupLabel, bs.Keystrokes, bs.Par, starLineShort(bs.Stars))
			b.WriteString(bestLabelStyle.Render(bsLine) + "\n")
		}

		// Mastery line.
		if mv, ok := m.progress.Mastery[key]; ok && mv.Rounds > 0 {
			pct := int(mv.Value * 100)
			mvLine := fmt.Sprintf("  %s  mastery: %d%% (%d rounds)",
				groupLabel, pct, mv.Rounds)
			b.WriteString(masteryLabelStyle.Render(mvLine) + "\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

// renderFooter renders the footer hotkey strip.
func (m LessonModel) renderFooter() string {
	b := m.config.Bindings
	retryLabel := fmt.Sprintf("%s: Retry", displayKey(b.Retry))
	hintLabel := fmt.Sprintf("%s: Hint", displayKey(b.Hint))
	skipLabel := fmt.Sprintf("%s: Skip", displayKey(b.Skip))
	pauseLabel := fmt.Sprintf("%s: Menu", displayKey(b.Pause))

	return footerStyle.Render(strings.Join([]string{retryLabel, hintLabel, skipLabel, pauseLabel}, "  |  "))
}

// renderUnlockProgress renders a thin progress bar toward the next unlock.
// It returns an empty string when all Motion Groups are unlocked.
func (m LessonModel) renderUnlockProgress() string {
	_, ratio := curriculum.FrontierProgress(m.masteryFloatMap())
	if ratio >= 1.0 {
		return ""
	}

	const barLen = 10
	filled := int(ratio * barLen)
	if filled < 0 {
		filled = 0
	}
	if filled > barLen {
		filled = barLen
	}

	return unlockBarFilledStyle.Render(strings.Repeat("▰", filled)) +
		unlockBarEmptyStyle.Render(strings.Repeat("▰", barLen-filled))
}

// masteryFloatMap extracts float64 mastery values from the progress map,
// keyed by group key, for use with curriculum.FrontierProgress.
func (m LessonModel) masteryFloatMap() map[string]float64 {
	result := make(map[string]float64, len(m.progress.Mastery))
	for k, v := range m.progress.Mastery {
		result[string(k)] = v.Value
	}
	return result
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
	case "ctrl+c":
		return "Ctrl-C"
	case "ctrl+h":
		return "Ctrl-H"
	case "ctrl+n":
		return "Ctrl-N"
	case "ctrl+r":
		return "Ctrl-R"
	default:
		return k
	}
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

// interpolate returns the cursor positions along the path from `from`
// to `to`, preserving order. For horizontal moves all intermediate
// columns are included; for vertical moves all intermediate rows are
// included (column clamped).
func interpolate(from, to vim.Cursor, buf vim.Buffer) []vim.Cursor {
	if from.Row == to.Row {
		// Horizontal: all columns from from.col to to.col in order.
		dir := 1
		if from.Col > to.Col {
			dir = -1
		}
		n := absDiff(from.Col, to.Col) + 1
		positions := make([]vim.Cursor, n)
		for i := 0; i < n; i++ {
			positions[i] = vim.Cursor{Row: from.Row, Col: from.Col + i*dir}
		}
		return positions
	}

	// Vertical: all rows from from.row to to.row in order.
	dir := 1
	if from.Row > to.Row {
		dir = -1
	}
	n := absDiff(from.Row, to.Row) + 1
	positions := make([]vim.Cursor, n)
	for i := 0; i < n; i++ {
		row := from.Row + i*dir
		col := from.Col
		if row < len(buf.Lines) {
			maxCol := len(buf.Lines[row])
			if maxCol > 0 {
				maxCol--
			}
			if col > maxCol {
				col = maxCol
			}
		}
		positions[i] = vim.Cursor{Row: row, Col: col}
	}
	// Ensure final position is exact.
	positions[n-1] = to
	return positions
}

// absDiff returns the absolute difference between two integers.
func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}