// Package tui renders the game state in the terminal and forwards
// keystrokes to the session. It is the only package that imports
// Bubble Tea / Charm libraries (ADR 0007).
package tui

import (
	"fmt"
	"math/rand"
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

	challengeCountStyle = lipgloss.NewStyle().
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
	ghostMaxLife  = 10 // frames before a ghost fades
	shakeFrames   = 6  // frames of shake animation on wasted key
	successFrames = 15 // frames of success burst animation
)

type tickMsg time.Time

// ghostPos tracks a single ghost trail cell.
type ghostPos struct {
	row, col int
	life     int // remaining animation frames
}

// ---------------------------------------------------------------------------
// RoundResult — one completed challenge in the session
// ---------------------------------------------------------------------------

// RoundResult holds the outcome of one challenge in the continuous stream.
type RoundResult struct {
	Template   challenge.TemplateKind
	Keystrokes int
	Par        int // -1 if unknown
	Stars      int
}

// ---------------------------------------------------------------------------
// ChallengeStream — generates challenges from a generator, weighted by
// frontier group
// ---------------------------------------------------------------------------

// ChallengeStream generates challenges on demand, filtering the generator's
// vocabulary to only include unlocked motions and weighting template
// selection toward the frontier group.
type ChallengeStream struct {
	gen *challenge.Generator
	rng *rand.Rand
	cfg challenge.Config
}

// defaultWallFraction is the proportion of regular stream rounds that are
// Wall Challenges targeting an already-unlocked (non-starting) group, when
// the caller doesn't specify one explicitly.
const defaultWallFraction = 1.0 / 3.0

// NewChallengeStream creates a stream that draws from gen with cfg.
func NewChallengeStream(gen *challenge.Generator, rng *rand.Rand, cfg challenge.Config) *ChallengeStream {
	if cfg.WallFraction == 0 {
		cfg.WallFraction = defaultWallFraction
	}
	return &ChallengeStream{gen: gen, rng: rng, cfg: cfg}
}

// Next generates a new challenge, weighted toward the frontier group. The
// generator's vocabulary is restricted to unlocked motions. The returned
// groupKey overrides curriculum.GroupForTemplate for progress tracking; it
// is non-empty only for Wall Challenges, which target a specific group
// regardless of the underlying template.
func (cs *ChallengeStream) Next(mastery map[string]float64) (c challenge.Challenge, tmpl challenge.TemplateKind, groupKey string, err error) {
	return cs.nextWithRetries(mastery, 3)
}

// nextWithRetries is like Next but with a bounded retry budget for
// challenges that aren't solvable with the restricted vocabulary.
func (cs *ChallengeStream) nextWithRetries(mastery map[string]float64, retries int) (challenge.Challenge, challenge.TemplateKind, string, error) {
	// Occasionally mix in a Wall Challenge targeting an already-unlocked,
	// non-starting group, to reinforce motions the player has learned.
	var wallEligible []string
	for i := 1; i < len(curriculum.Groups); i++ {
		if mastery[curriculum.Groups[i].Key] >= curriculum.MasteryThreshold {
			wallEligible = append(wallEligible, curriculum.Groups[i].Key)
		}
	}
	if len(wallEligible) > 0 && cs.rng.Float64() < cs.cfg.WallFraction {
		groupKey := wallEligible[cs.rng.Intn(len(wallEligible))]
		if group := curriculum.GroupForGroupKey(groupKey); group != nil {
			if c, err := cs.gen.GenerateWall(groupKey, group.Keys, cs.cfg); err == nil {
				return c, challenge.THorizontalLine, groupKey, nil
			}
		}
		// Fall through to normal generation on failure.
	}

	// Determine the frontier group.
	frontierIdx, _ := curriculum.FrontierProgress(mastery)

	// Build template weights: frontier templates get higher weight.
	type tmplWeight struct {
		tmpl   challenge.TemplateKind
		weight int
	}

	var candidates []tmplWeight
	allTemplates := challenge.Templates()

	if frontierIdx >= 0 {
		frontierKey := curriculum.Groups[frontierIdx].Key
		frontierTemplates := curriculum.TemplatesForGroup(frontierKey)
		// Frontier templates weighted 3:1 over non-frontier.
		for _, t := range allTemplates {
			w := 1
			for _, ft := range frontierTemplates {
				if t == ft {
					w = 3
					break
				}
			}
			candidates = append(candidates, tmplWeight{tmpl: t, weight: w})
		}
	} else {
		// All groups unlocked — equal weight.
		for _, t := range allTemplates {
			candidates = append(candidates, tmplWeight{tmpl: t, weight: 1})
		}
	}

	// Weighted random selection.
	totalWeight := 0
	for _, c := range candidates {
		totalWeight += c.weight
	}
	roll := cs.rng.Intn(totalWeight)
	cumulative := 0
	var picked challenge.TemplateKind
	for _, c := range candidates {
		cumulative += c.weight
		if roll < cumulative {
			picked = c.tmpl
			break
		}
	}

	// Build the unlocked vocabulary.
	unlocked := curriculum.UnlockedVocabulary(mastery)

	// Generate a challenge using the generator's (full) vocabulary.
	c, err := cs.gen.Generate(picked, cs.cfg)
	if err != nil {
		return challenge.Challenge{}, challenge.TemplateKind(0), "", fmt.Errorf("generate %s: %w", picked, err)
	}

	// If the unlocked vocabulary is a strict subset of the full set,
	// re-solve to verify solvability with only unlocked motions.
	if len(unlocked) < len(challenge.NavigationVocabulary) {
		par := cs.gen.Solver().Solve(c, unlocked, cs.gen.MaxDepth())
		if par < 0 {
			// Not solvable with unlocked motions — retry if budget allows.
			if retries > 0 {
				return cs.nextWithRetries(mastery, retries-1)
			}
			return challenge.Challenge{}, challenge.TemplateKind(0), "",
				fmt.Errorf("challenge not solvable with unlocked vocabulary after retries")
		}
		c.Par = par
	}

	return c, picked, "", nil
}

// NextIntro generates a challenge that specifically exercises newVocab
// (typically prevVocab plus a newly unlocked group): it retries until it
// finds a challenge that newVocab solves in fewer keystrokes than prevVocab,
// proving the new motions actually help.
func (cs *ChallengeStream) NextIntro(prevVocab, newVocab []string) (challenge.Challenge, challenge.TemplateKind, error) {
	const maxAttempts = 100
	introCfg := cs.cfg
	if introCfg.MinDistance < 10 {
		introCfg.MinDistance = 10
	}
	introCfg.MinMotions = 0

	templates := challenge.Templates()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		tmpl := templates[cs.rng.Intn(len(templates))]
		c, err := cs.gen.Generate(tmpl, introCfg)
		if err != nil {
			continue
		}

		oldPar := cs.gen.Solver().Solve(c, prevVocab, cs.gen.MaxDepth())
		newPar := cs.gen.Solver().Solve(c, newVocab, cs.gen.MaxDepth())
		if oldPar < 0 || newPar <= 0 || oldPar <= newPar {
			continue // new vocabulary doesn't help solve this challenge
		}

		c.Par = newPar
		return c, tmpl, nil
	}

	return challenge.Challenge{}, challenge.TemplateKind(0), fmt.Errorf("unable to generate intro round after %d attempts", maxAttempts)
}

// subtractKeys returns the elements of vocab that are not present in remove.
func subtractKeys(vocab, remove []string) []string {
	removeSet := make(map[string]bool, len(remove))
	for _, k := range remove {
		removeSet[k] = true
	}
	result := make([]string, 0, len(vocab))
	for _, k := range vocab {
		if !removeSet[k] {
			result = append(result, k)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// GameModel — renders one challenge (round)
// ---------------------------------------------------------------------------

// GameModel is the Bubble Tea model for a single challenge round.
type GameModel struct {
	session    *session.Session
	challenge  challenge.Challenge
	goalRow    int
	goalCol    int
	keystrokes []string

	// Ghost trail
	prevCursor vim.Cursor
	ghosts     []ghostPos

	// Cache: optimal distance from previous step (avoids redundant solver call).
	// Initialised in NewGame.
	lastOptimal int

	// Effects
	wasteFrames   int    // non-zero when a wasted-key shake is active
	successFrames int    // non-zero when a success burst is playing
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

var pauseMenuItems = []string{"Resume", "Skip", "Retry", "Restart Session", "Quit"}

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
// StreamModel — orchestrates a continuous stream of challenges
// ---------------------------------------------------------------------------

// streamState tracks where we are in the challenge stream.
type streamState int

const (
	statePlaying            streamState = iota // a challenge is in progress
	stateUnlockInterstitial                    // showing unlock interstitial for a new motion group
	stateIntroRound                            // playing the forced intro round for a newly unlocked group
	stateSummary                               // showing session summary (on quit)
)

// StreamModel is the Bubble Tea model for a continuous challenge stream.
type StreamModel struct {
	game            GameModel
	state           streamState
	currentTemplate challenge.TemplateKind

	// currentGroupKey overrides curriculum.GroupForTemplate(currentTemplate)
	// for progress tracking. Non-empty only for Wall Challenges, which
	// target a specific motion group regardless of the underlying template.
	currentGroupKey string

	// Pause overlay
	paused     bool
	menuCursor int

	// Configuration
	config Config

	// Challenge generation
	stream *ChallengeStream

	// Session tracking
	completed []RoundResult // completed challenges this session

	// Progress persistence (may be nil)
	store    store.Store
	progress store.Progress // in-memory copy of progress (updated live)

	// Unlock interstitial / intro round
	pendingGroup curriculum.MotionGroup // group to be introduced via interstitial+intro round
	introGame    GameModel              // game model for the intro round
}

// NewStream creates a StreamModel that generates challenges from the given
// generator and persists progress through the store.
func NewStream(gen *challenge.Generator, rng *rand.Rand, s store.Store, cfg ...Config) StreamModel {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

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

	stream := NewChallengeStream(gen, rng, challenge.DefaultConfig())
	c, tmpl, groupKey, err := stream.Next(masteryFloatMap(p.Mastery))
	if err != nil {
		// Fallback: use full vocabulary to generate a challenge.
		c, err = gen.Generate(challenge.THorizontalLine, challenge.DefaultConfig())
		if err != nil {
			panic("failed to generate initial challenge: " + err.Error())
		}
		tmpl = challenge.THorizontalLine
		groupKey = ""
	}

	return StreamModel{
		game:            NewGame(c, config.Bindings, config.ReducedMotion),
		currentTemplate: tmpl,
		currentGroupKey: groupKey,
		state:           statePlaying,
		config:          config,
		stream:          stream,
		completed:       make([]RoundResult, 0, 64),
		store:           s,
		progress:        p,
	}
}

// Init implements tea.Model.
func (m StreamModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m StreamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Advance animations during any non-paused state.
		if m.paused || m.state == stateSummary {
			return m, nil
		}

		// Advance animations for whichever game is active.
		if m.state == stateIntroRound {
			m.introGame.advanceAnimations()
			active := m.introGame.wasteFrames > 0 || m.introGame.successFrames > 0 || len(m.introGame.ghosts) > 0
			if active {
				return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg {
					return tickMsg(t)
				})
			}
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
		// Handle pause toggle globally.
		if msg.String() == m.config.Bindings.Pause {
			canPause := m.paused ||
				m.state == statePlaying ||
				m.state == stateIntroRound ||
				m.state == stateUnlockInterstitial
			if canPause {
				m.paused = !m.paused
				m.menuCursor = 0
				return m, nil
			}
			return m, nil
		}

		// Handle menu navigation when paused.
		if m.paused {
			cmd := m.handleMenuKey(msg)
			return m, cmd
		}

		// Esc is never intercepted by the app (reserved for vim).
		if msg.String() == "esc" {
			if m.state == statePlaying || m.state == stateIntroRound {
				var cmd tea.Cmd
				if m.state == stateIntroRound {
					m.introGame, cmd = m.introGame.Update(msg)
				} else {
					m.game, cmd = m.game.Update(msg)
				}
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

		case m.state == stateUnlockInterstitial:
			// Acknowledge the interstitial to start the intro round.
			if msg.String() == " " || msg.String() == "enter" {
				m.startIntroRound()
				return m, nil
			}
			return m, nil

		case m.state == stateIntroRound && (msg.String() == " " || msg.String() == "enter"):
			if m.introGame.Solved() {
				m.afterIntroRound()
				return m, nil
			}
			return m, nil

		case msg.String() == m.config.Bindings.Hint:
			if m.state == statePlaying && !m.game.Solved() {
				m.showHint()
			} else if m.state == stateIntroRound && !m.introGame.Solved() {
				m.showHint()
			}
			return m, nil

		case msg.String() == m.config.Bindings.Retry:
			if m.state == statePlaying {
				m.retryChallenge()
				return m, nil
			}

		case msg.String() == m.config.Bindings.Skip:
			m.skipChallenge()
			return m, nil
		}

		// Forward keystrokes to the active game.
		switch m.state {
		case statePlaying:
			var cmd tea.Cmd
			m.game, cmd = m.game.Update(msg)
			// Check if the round just completed.
			if m.game.Solved() {
				m.onRoundSolved()
			}
			return m, cmd

		case stateIntroRound:
			var cmd tea.Cmd
			m.introGame, cmd = m.introGame.Update(msg)
			if m.introGame.Solved() {
				m.onIntroRoundSolved()
			}
			return m, cmd
		}
	}

	return m, nil
}

// onRoundSolved handles completion of a normal stream round: records the
// result, persists progress, and either starts the unlock interstitial (if
// this round pushed the frontier group's mastery across the threshold) or
// auto-advances to the next challenge.
func (m *StreamModel) onRoundSolved() {
	r := m.game.Result()

	m.completed = append(m.completed, RoundResult{
		Template:   m.currentTemplate,
		Keystrokes: r.Keystrokes,
		Par:        r.Par,
		Stars:      r.Stars,
	})

	beforeIdx, _ := curriculum.FrontierProgress(masteryFloatMap(m.progress.Mastery))

	groupKey := m.currentGroupKey
	if groupKey == "" {
		groupKey = curriculum.GroupForTemplate(m.currentTemplate)
	}
	m.updateProgress(groupKey, r)
	m.saveProgress()

	afterIdx, _ := curriculum.FrontierProgress(masteryFloatMap(m.progress.Mastery))

	if beforeIdx >= 0 && afterIdx != beforeIdx {
		// The frontier group just crossed the mastery threshold.
		m.pendingGroup = curriculum.Groups[beforeIdx]
		m.state = stateUnlockInterstitial
		return
	}

	m.nextChallenge()
}

// startIntroRound generates a challenge that specifically exercises the
// newly-unlocked group and starts the forced intro round.
func (m *StreamModel) startIntroRound() {
	newVocab := curriculum.UnlockedVocabulary(masteryFloatMap(m.progress.Mastery))
	prevVocab := subtractKeys(newVocab, m.pendingGroup.Keys)

	c, tmpl, err := m.stream.NextIntro(prevVocab, newVocab)
	if err != nil {
		// Fallback: a normal round instead.
		m.nextChallenge()
		return
	}
	m.introGame = NewGame(c, m.config.Bindings, m.config.ReducedMotion)
	m.currentTemplate = tmpl
	m.currentGroupKey = ""
	m.state = stateIntroRound
}

// onIntroRoundSolved records the intro round's result. The player must
// press space/enter afterward (afterIntroRound) to move on, giving the
// unlock its own beat.
func (m *StreamModel) onIntroRoundSolved() {
	r := m.introGame.Result()
	m.completed = append(m.completed, RoundResult{
		Template:   m.currentTemplate,
		Keystrokes: r.Keystrokes,
		Par:        r.Par,
		Stars:      r.Stars,
	})
	groupKey := curriculum.GroupForTemplate(m.currentTemplate)
	m.updateProgress(groupKey, r)
	m.saveProgress()
}

// afterIntroRound handles the transition after acknowledging a solved intro round.
func (m *StreamModel) afterIntroRound() {
	m.nextChallenge()
}

// updateProgress updates the in-memory progress with the round result.
func (m *StreamModel) updateProgress(groupKey string, r session.Result) {
	key := store.GroupKey(groupKey)

	// Update best score.
	current := m.progress.BestScores[key]
	m.progress.BestScores[key] = store.UpdateBestScore(current, r.Keystrokes, r.Par, r.Stars)

	// Update mastery EWMA.
	prev := m.progress.Mastery[key]
	m.progress.Mastery[key] = store.UpdateMastery(prev, r.Keystrokes, r.Par, r.Stars, store.DefaultAlpha)
}

// saveProgress persists the current in-memory progress through the store.
func (m *StreamModel) saveProgress() {
	if m.store != nil {
		_ = m.store.SaveProgress(m.progress)
	}
}

// handleMenuKey processes key events while the pause overlay is active.
func (m *StreamModel) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
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

// executeMenuSelection runs the action for the currently selected menu item.
func (m *StreamModel) executeMenuSelection() tea.Cmd {
	item := pauseMenuItems[m.menuCursor]
	switch item {
	case "Resume":
		m.paused = false
		return nil
	case "Skip":
		m.paused = false
		m.skipChallenge()
		return nil
	case "Retry":
		m.paused = false
		m.retryChallenge()
		return nil
	case "Restart Session":
		m.paused = false
		m.restartSession()
		return nil
	case "Quit":
		m.showSummary()
		return nil
	}
	return nil
}

// showHint computes and displays the optimal next keystroke. Works for both
// the normal and intro round games.
func (m *StreamModel) showHint() {
	g := &m.game
	if m.state == stateIntroRound {
		g = &m.introGame
	}
	state := g.session.State()
	vocab := curriculum.UnlockedVocabulary(masteryFloatMap(m.progress.Mastery))
	maxDepth := solver.DefaultMaxDepth
	key, _ := solver.FirstStepFromState(state, g.challenge, vocab, maxDepth)
	if key != "" {
		g.hintKey = key
		g.hintVisible = true
	}
}

// skipChallenge moves to the next challenge, recording the current one
// (normal or intro round) as skipped.
func (m *StreamModel) skipChallenge() {
	g := &m.game
	if m.state == stateIntroRound {
		g = &m.introGame
	}
	m.completed = append(m.completed, RoundResult{
		Template:   m.currentTemplate,
		Keystrokes: 0,
		Par:        g.challenge.Par,
		Stars:      0,
	})
	m.nextChallenge()
}

// retryChallenge restarts the current challenge.
func (m *StreamModel) retryChallenge() {
	m.game = NewGame(m.game.challenge, m.config.Bindings, m.config.ReducedMotion)
	m.state = statePlaying
}

// nextChallenge generates and starts a new challenge.
func (m *StreamModel) nextChallenge() {
	c, tmpl, groupKey, err := m.stream.Next(masteryFloatMap(m.progress.Mastery))
	if err != nil {
		// Fallback: retry once with full vocabulary.
		gen := challenge.NewGenerator(
			rand.New(rand.NewSource(time.Now().UnixNano())),
			challenge.SolverFunc(solver.Solve),
			nil,
			solver.DefaultMaxDepth,
		)
		c, err = gen.Generate(challenge.THorizontalLine, challenge.DefaultConfig())
		if err != nil {
			// Cannot generate — show summary.
			m.showSummary()
			return
		}
		tmpl = challenge.THorizontalLine
		groupKey = ""
	}
	m.game = NewGame(c, m.config.Bindings, m.config.ReducedMotion)
	m.currentTemplate = tmpl
	m.currentGroupKey = groupKey
	m.state = statePlaying
}

// restartSession clears all session results and starts fresh.
func (m *StreamModel) restartSession() {
	m.completed = make([]RoundResult, 0, 64)
	m.nextChallenge()
}

// showSummary transitions to the summary screen (saving progress first).
func (m *StreamModel) showSummary() {
	m.saveProgress()
	m.state = stateSummary
	m.paused = false
}

// ---------------------------------------------------------------------------
// Unlock interstitial styling
// ---------------------------------------------------------------------------

var (
	unlockTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#0ff"))

	unlockGroupStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#ff0")).
				Background(lipgloss.Color("#228"))

	unlockPitchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ccc"))

	unlockPromptStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#888"))

	interstitialBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#0ff")).
				Padding(2, 4).
				Width(50)
)

// viewUnlockInterstitial renders the full-screen unlock interstitial.
func (m StreamModel) viewUnlockInterstitial() string {
	var b strings.Builder

	b.WriteString(unlockTitleStyle.Render("★ New Unlock! ★") + "\n\n")

	b.WriteString(unlockGroupStyle.Render("  "+m.pendingGroup.Name+"  ") + "\n\n")

	b.WriteString(unlockPitchStyle.Render(m.pendingGroup.Pitch) + "\n\n\n")

	b.WriteString(unlockPromptStyle.Render("Press space for the intro round."))

	content := b.String()

	// Center the content in a bordered box.
	return interstitialBorder.Render(content)
}

// viewIntroRound renders the intro round game.
func (m StreamModel) viewIntroRound() string {
	var groupLine string
	groupLine = unlockGroupStyle.Render("  " + m.pendingGroup.Name + "  ")

	gameView := m.introGame.ViewGame()

	var goalLine string
	if !m.introGame.Solved() {
		goalLine = normalStyle.Render("Use the new motions!")
	}

	footer := m.renderFooter()

	parts := []string{
		groupLine,
		"",
		gameView,
		"",
		goalLine,
		"",
		footer,
	}
	return strings.Join(parts, "\n")
}

// View implements tea.Model.
func (m StreamModel) View() string {
	if m.paused {
		return m.viewPaused()
	}

	switch m.state {
	case statePlaying:
		return m.viewPlaying()
	case stateUnlockInterstitial:
		return m.viewUnlockInterstitial()
	case stateIntroRound:
		return m.viewIntroRound()
	case stateSummary:
		return m.viewSummary()
	default:
		return normalStyle.Render("unknown state")
	}
}

// viewPlaying renders the current challenge with footer.
func (m StreamModel) viewPlaying() string {
	header := headerStyle.Render(fmt.Sprintf("Challenge %d", len(m.completed)+1))
	tmplLine := templateStyle.Render(m.currentTemplate.String())
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

// viewPaused renders the pause overlay.
func (m StreamModel) viewPaused() string {
	var content string
	switch m.state {
	case statePlaying:
		content = m.viewPlaying()
	case stateUnlockInterstitial:
		content = m.viewUnlockInterstitial()
	case stateIntroRound:
		content = m.viewIntroRound()
	default:
		content = m.viewPlaying()
	}

	overlay := viewPauseOverlay(m.menuCursor)
	return content + "\n\n" + overlay
}

// viewSummary renders the session summary screen.
func (m StreamModel) viewSummary() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Practice Complete") + "\n\n")

	totalKeystrokes := 0
	totalPar := 0
	parCount := 0

	for i, r := range m.completed {
		challengeLabel := challengeCountStyle.Render(fmt.Sprintf("Challenge %d", i+1))
		tmplLabel := templateStyle.Render(r.Template.String())

		var resultLine string
		if r.Keystrokes > 0 {
			totalKeystrokes += r.Keystrokes
			if r.Par >= 0 {
				totalPar += r.Par
				parCount++
				resultLine = fmt.Sprintf("  %s  %s\n  you %d — par %d",
					challengeLabel, tmplLabel, r.Keystrokes, r.Par)
			} else {
				resultLine = fmt.Sprintf("  %s  %s\n  you %d",
					challengeLabel, tmplLabel, r.Keystrokes)
			}
		} else {
			resultLine = fmt.Sprintf("  %s  %s  — skipped", challengeLabel, tmplLabel)
		}
		b.WriteString(resultLine + "\n\n")
	}

	// Aggregate.
	b.WriteString(summaryLabelStyle.Render("Session Total") + "\n")
	totalLine := fmt.Sprintf("  challenges: %d  keystrokes: %d",
		len(m.completed), totalKeystrokes)
	b.WriteString(totalLine + "\n\n")

	b.WriteString(normalStyle.Render("Press q to quit."))
	return b.String()
}

// renderFooter renders the footer hotkey strip.
func (m StreamModel) renderFooter() string {
	b := m.config.Bindings
	retryLabel := fmt.Sprintf("%s: Retry", displayKey(b.Retry))
	hintLabel := fmt.Sprintf("%s: Hint", displayKey(b.Hint))
	skipLabel := fmt.Sprintf("%s: Skip", displayKey(b.Skip))
	pauseLabel := fmt.Sprintf("%s: Menu", displayKey(b.Pause))

	return footerStyle.Render(strings.Join([]string{retryLabel, hintLabel, skipLabel, pauseLabel}, "  |  "))
}

// renderUnlockProgress renders a thin progress bar toward the next unlock.
// It returns an empty string when all Motion Groups are unlocked.
func (m StreamModel) renderUnlockProgress() string {
	idx, ratio := curriculum.FrontierProgress(masteryFloatMap(m.progress.Mastery))
	if idx < 0 {
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
func masteryFloatMap(mastery map[store.GroupKey]store.Mastery) map[string]float64 {
	result := make(map[string]float64, len(mastery))
	for k, v := range mastery {
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
// predicate.
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
// to `to`, preserving order.
func interpolate(from, to vim.Cursor, buf vim.Buffer) []vim.Cursor {
	if from.Row == to.Row {
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
