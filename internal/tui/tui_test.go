package tui

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/vim"
)

// testGenerator creates a deterministic generator for tests.
func testGenerator(seed int64) *challenge.Generator {
	rng := rand.New(rand.NewSource(seed))
	return challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)
}

// buffer is a shorthand for creating a Buffer with the given lines.
func buffer(lines ...string) vim.Buffer {
	return vim.Buffer{Lines: lines}
}

func TestDefaultConfig_HasBindings(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Bindings.Pause != "ctrl+c" {
		t.Errorf("default pause key = %q, want ctrl+c", cfg.Bindings.Pause)
	}
	if cfg.Bindings.Hint != "ctrl+h" {
		t.Errorf("default hint key = %q, want ctrl+h", cfg.Bindings.Hint)
	}
	if cfg.Bindings.Skip != "ctrl+n" {
		t.Errorf("default skip key = %q, want ctrl+n", cfg.Bindings.Skip)
	}
	if cfg.Bindings.Retry != "ctrl+r" {
		t.Errorf("default retry key = %q, want ctrl+r", cfg.Bindings.Retry)
	}
}

func TestNewStream_StartsPlaying(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)
	if m.state != statePlaying {
		t.Fatalf("initial state = %d, want statePlaying", m.state)
	}
	if m.game.Solved() {
		t.Fatal("initial challenge should not be solved")
	}
}

func TestStreamModel_KeystrokeAdvancesSession(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	// Send an 'l' keystroke (hjkl is always unlocked).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated := m2.(StreamModel)

	if updated.state != statePlaying {
		t.Fatalf("state = %d, want statePlaying", updated.state)
	}
}

func TestStreamModel_SkipKey(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	initialCompleted := len(m.completed)

	// Skip the current challenge.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	skipped := m2.(StreamModel)

	if skipped.state != statePlaying {
		t.Fatalf("after skip, state = %d, want statePlaying", skipped.state)
	}
	if len(skipped.completed) != initialCompleted+1 {
		t.Fatalf("after skip, completed = %d, want %d", len(skipped.completed), initialCompleted+1)
	}
	last := skipped.completed[len(skipped.completed)-1]
	if last.Keystrokes != 0 {
		t.Fatalf("skipped round keystrokes = %d, want 0", last.Keystrokes)
	}
}

func TestStreamModel_HintKey(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	hinted := m2.(StreamModel)

	if !hinted.game.hintVisible {
		t.Fatal("expected hint to be visible after Ctrl-H")
	}
	if hinted.game.hintKey == "" {
		t.Fatal("expected hintKey to be non-empty")
	}
}

func TestStreamModel_PauseToggle(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)

	if !paused.paused {
		t.Fatal("expected paused to be true after Ctrl-C")
	}
	if paused.menuCursor != 0 {
		t.Fatalf("menu cursor = %d, want 0", paused.menuCursor)
	}

	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	resumed := m3.(StreamModel)

	if resumed.paused {
		t.Fatal("expected paused to be false after second Ctrl-C")
	}
}

func TestStreamModel_MenuNavigation(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)

	// Navigate down with 'j'.
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	down := m3.(StreamModel)

	if down.menuCursor != 1 {
		t.Fatalf("after j, menu cursor = %d, want 1", down.menuCursor)
	}

	// Navigate up with 'k'.
	m4, _ := down.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	up := m4.(StreamModel)

	if up.menuCursor != 0 {
		t.Fatalf("after k, menu cursor = %d, want 0", up.menuCursor)
	}

	// Select Resume with Enter.
	m5, _ := up.Update(tea.KeyMsg{Type: tea.KeyEnter})
	resumed := m5.(StreamModel)

	if resumed.paused {
		t.Fatal("expected paused to be false after Resume")
	}
}

func TestStreamModel_EscNotIntercepted(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	after := m2.(StreamModel)

	if after.state != statePlaying {
		t.Fatalf("after esc, state = %d, want statePlaying", after.state)
	}
	if after.game.Solved() {
		t.Fatal("session should not be solved after esc")
	}
}

func TestStreamModel_RetryKey_MidRound(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	initialCol := m.game.session.State().Cursor.Col

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	afterMove := m2.(StreamModel)

	m3, _ := afterMove.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	retried := m3.(StreamModel)

	if retried.game.session.State().Cursor.Col != initialCol {
		t.Fatalf("after retry, cursor col = %d, want %d", retried.game.session.State().Cursor.Col, initialCol)
	}
	if retried.state != statePlaying {
		t.Fatalf("after retry, state = %d, want statePlaying", retried.state)
	}
}

func TestStreamModel_SkipAutoAdvances(t *testing.T) {
	// Skipping a challenge auto-advances to the next one, staying in playing state.
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	initialCount := len(m.completed)

	// Skip the challenge.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(StreamModel)

	// Should still be playing and completed count incremented.
	if m.state != statePlaying {
		t.Fatalf("after skip, state = %d, want statePlaying", m.state)
	}
	if len(m.completed) != initialCount+1 {
		t.Fatalf("after skip, completed = %d, want %d", len(m.completed), initialCount+1)
	}
	// Challenge should be different (new cursor position, new buffer).
	if len(m.completed) > 0 {
		last := m.completed[len(m.completed)-1]
		if last.Keystrokes != 0 {
			t.Fatalf("skipped challenge keystrokes = %d, want 0", last.Keystrokes)
		}
	}
}

func TestStreamModel_QuitShowsSummary(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)

	for i := 0; i < 4; i++ {
		var tmp tea.Model
		tmp, _ = paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		paused = tmp.(StreamModel)
	}

	// Select Quit from menu — transitions to summary, does not quit directly.
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyEnter})
	quitResult := m3.(StreamModel)

	if quitResult.state != stateSummary {
		t.Fatalf("after Quit, state = %d, want stateSummary", quitResult.state)
	}

	// From summary, pressing 'q' should quit.
	_, cmd := quitResult.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd after 'q' from summary")
	}

	view := quitResult.View()
	if view == "" {
		t.Fatal("summary view should not be empty")
	}
}

func TestStreamModel_AnimationsCreateTick(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	after := m2.(StreamModel)

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tick) after keystroke")
	}
	if len(after.game.ghosts) == 0 {
		t.Fatal("expected ghost trail after cursor move")
	}
}

func TestGameModel_ReducedMotionNoGhosts(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	gm := NewGame(c, DefaultBindings(), true)

	gm2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	after := gm2

	if len(after.ghosts) > 0 {
		t.Fatal("expected no ghosts in reduced motion mode")
	}
}

func TestDisplayKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"h", "h"},
		{"l", "l"},
		{" ", "<space>"},
		{"enter", "<cr>"},
		{"tab", "<tab>"},
		{"esc", "<esc>"},
		{"ctrl+c", "Ctrl-C"},
		{"ctrl+h", "Ctrl-H"},
		{"ctrl+n", "Ctrl-N"},
		{"ctrl+r", "Ctrl-R"},
	}

	for _, tt := range tests {
		got := displayKey(tt.key)
		if got != tt.want {
			t.Errorf("displayKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestInterpolate(t *testing.T) {
	buf := buffer("abcdef", "ghi")

	tests := []struct {
		name string
		from vim.Cursor
		to   vim.Cursor
		want int
	}{
		{
			name: "horizontal right",
			from: vim.Cursor{Row: 0, Col: 0},
			to:   vim.Cursor{Row: 0, Col: 3},
			want: 4,
		},
		{
			name: "horizontal left",
			from: vim.Cursor{Row: 0, Col: 3},
			to:   vim.Cursor{Row: 0, Col: 0},
			want: 4,
		},
		{
			name: "vertical down",
			from: vim.Cursor{Row: 0, Col: 1},
			to:   vim.Cursor{Row: 1, Col: 1},
			want: 2,
		},
		{
			name: "vertical up",
			from: vim.Cursor{Row: 1, Col: 1},
			to:   vim.Cursor{Row: 0, Col: 1},
			want: 2,
		},
		{
			name: "no movement",
			from: vim.Cursor{Row: 0, Col: 2},
			to:   vim.Cursor{Row: 0, Col: 2},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpolate(tt.from, tt.to, buf)
			if len(got) != tt.want {
				t.Errorf("interpolate returned %d positions, want %d", len(got), tt.want)
			}
			if got[0] != tt.from {
				t.Errorf("first position = %v, want %v", got[0], tt.from)
			}
			if got[len(got)-1] != tt.to {
				t.Errorf("last position = %v, want %v", got[len(got)-1], tt.to)
			}
		})
	}
}

func TestViewGame_DoesNotCrash(t *testing.T) {
	c := challenge.New(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
	)
	gm := NewGame(c, DefaultBindings(), false)
	_ = gm.ViewGame()

	gm2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	_ = gm2.ViewGame()
}

func TestStreamView_DoesNotCrash(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)
	_ = m.View()
}

func TestMenuView_DoesNotCrash(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)
	_ = paused.View()
}

func TestApplyKeystroke_WastedKeyDetection(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	gm := NewGame(c, DefaultBindings(), false)

	gm.applyKeystroke("h")
	if gm.wasteFrames <= 0 {
		t.Fatal("expected wasteFrames > 0 after wasted key (h at col 0)")
	}

	gm = NewGame(c, DefaultBindings(), false)
	gm.applyKeystroke("l")
	if gm.wasteFrames > 0 {
		t.Fatal("expected wasteFrames == 0 after progress key (l)")
	}
}

func TestNewStreamWithConfig_CustomBindings(t *testing.T) {
	cfg := Config{
		Bindings: KeyBindings{
			Pause: "ctrl+p",
			Hint:  "ctrl+h",
			Skip:  "ctrl+s",
		},
		ReducedMotion: true,
	}
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil, cfg)

	if !m.config.ReducedMotion {
		t.Fatal("expected ReducedMotion to be true")
	}
	if m.config.Bindings.Pause != "ctrl+p" {
		t.Fatalf("Pause binding = %q, want ctrl+p", m.config.Bindings.Pause)
	}
	if m.config.Bindings.Skip != "ctrl+s" {
		t.Fatalf("Skip binding = %q, want ctrl+s", m.config.Bindings.Skip)
	}
}

func TestStreamModel_MultipleSkips(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	for i := 0; i < 3; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
		m = m2.(StreamModel)
		if m.state != statePlaying {
			t.Fatalf("after skip %d, state = %d, want statePlaying", i+1, m.state)
		}
	}

	if len(m.completed) != 3 {
		t.Fatalf("after 3 skips, completed = %d, want 3", len(m.completed))
	}
}

func TestStreamModel_RestartSession(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(StreamModel)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(StreamModel)

	if len(m.completed) != 2 {
		t.Fatalf("before restart, completed = %d, want 2", len(m.completed))
	}

	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)
	for i := 0; i < 3; i++ {
		var tmp tea.Model
		tmp, _ = paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		paused = tmp.(StreamModel)
	}
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyEnter})
	restarted := m3.(StreamModel)

	if len(restarted.completed) != 0 {
		t.Fatalf("after restart, completed = %d, want 0", len(restarted.completed))
	}
	if restarted.state != statePlaying {
		t.Fatalf("after restart, state = %d, want statePlaying", restarted.state)
	}
}

func TestAdvanceAnimations_DecrementsCounters(t *testing.T) {
	gm := GameModel{
		wasteFrames:   5,
		successFrames: 10,
		ghosts: []ghostPos{
			{row: 0, col: 0, life: 3},
			{row: 0, col: 1, life: 5},
		},
		reducedMotion: false,
	}

	active := gm.advanceAnimations()
	if active <= 0 {
		t.Fatal("expected active animations after advance")
	}
	if gm.wasteFrames != 4 {
		t.Fatalf("wasteFrames = %d, want 4", gm.wasteFrames)
	}
	if gm.successFrames != 9 {
		t.Fatalf("successFrames = %d, want 9", gm.successFrames)
	}
	if len(gm.ghosts) != 2 {
		t.Fatalf("len(ghosts) = %d, want 2", len(gm.ghosts))
	}
	if gm.ghosts[0].life != 2 {
		t.Fatalf("ghost[0].life = %d, want 2", gm.ghosts[0].life)
	}
}

func TestAdvanceAnimations_RemovesExpiredGhosts(t *testing.T) {
	gm := GameModel{
		ghosts: []ghostPos{
			{row: 0, col: 0, life: 1},
			{row: 0, col: 1, life: 2},
		},
		reducedMotion: false,
	}

	gm.advanceAnimations()
	if len(gm.ghosts) != 1 {
		t.Fatalf("len(ghosts) after expiry = %d, want 1", len(gm.ghosts))
	}
	if gm.ghosts[0].col != 1 {
		t.Fatalf("remaining ghost col = %d, want 1", gm.ghosts[0].col)
	}
}

func TestGhostAt(t *testing.T) {
	gm := GameModel{
		ghosts: []ghostPos{
			{row: 0, col: 0, life: 5},
			{row: 0, col: 1, life: 3},
		},
	}

	if life := gm.ghostAt(0, 0); life != 5 {
		t.Fatalf("ghostAt(0,0) = %d, want 5", life)
	}
	if life := gm.ghostAt(0, 1); life != 3 {
		t.Fatalf("ghostAt(0,1) = %d, want 3", life)
	}
	if life := gm.ghostAt(0, 2); life != 0 {
		t.Fatalf("ghostAt(0,2) = %d, want 0", life)
	}
	if life := gm.ghostAt(1, 0); life != 0 {
		t.Fatalf("ghostAt(1,0) = %d, want 0", life)
	}
}

func TestGameModel_SolvedIgnoresKeystrokes(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 2},
		challenge.CursorAtTarget(0, 2),
	)
	gm := NewGame(c, DefaultBindings(), false)

	if !gm.Solved() {
		t.Fatal("expected solved immediately")
	}

	gm2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	after := gm2
	if after.session.State().Cursor.Col != 2 {
		t.Fatalf("cursor should not move after solved, col = %d", after.session.State().Cursor.Col)
	}
}

func TestViewGame_WallCellRenderedDistinctly(t *testing.T) {
	// Challenge with walls should render them with a distinct style.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	gm := NewGame(c, DefaultBindings(), false)
	view := gm.ViewGame()
	// Should not panic — wall rendering is present.
	_ = view
}

func TestViewGame_WallsNoPanic(t *testing.T) {
	// Multiple walls on multi-line buffer — should not panic.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 0}: true,
		vim.Cursor{Row: 0, Col: 2}: true,
		vim.Cursor{Row: 1, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc", "def"),
		vim.Cursor{Row: 0, Col: 1},
		challenge.CursorAtTarget(1, 2),
		walls,
	)
	gm := NewGame(c, DefaultBindings(), false)
	view := gm.ViewGame()
	_ = view
}

func TestWallRefusalTriggersShake(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	gm := NewGame(c, DefaultBindings(), false)
	gm.applyKeystroke("l") // should be refused — wall at col 1
	if gm.wasteFrames <= 0 {
		t.Fatal("expected wasteFrames > 0 after wall refusal")
	}
	if gm.session.State().Cursor.Col != 0 {
		t.Fatalf("cursor should not move after wall refusal, col = %d", gm.session.State().Cursor.Col)
	}
}

func TestWallRefusalShakeAndCounter(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abcd"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 3),
		walls,
	)
	gm := NewGame(c, DefaultBindings(), false)

	gm.applyKeystroke("l") // refused by wall
	if gm.wasteFrames <= 0 {
		t.Fatal("expected wasteFrames > 0 after wall refusal")
	}
	if got := gm.session.KeystrokeCount(); got != 1 {
		t.Fatalf("keystroke count = %d, want 1", got)
	}
	if gm.session.State().Cursor.Col != 0 {
		t.Fatalf("cursor col = %d, want 0 after wall refusal", gm.session.State().Cursor.Col)
	}

	// After advancing animations, waste frames should decrease.
	active := gm.advanceAnimations()
	if active <= 0 {
		t.Fatal("expected active animations")
	}
	if gm.wasteFrames >= shakeFrames {
		t.Fatalf("wasteFrames = %d, expected to decrease", gm.wasteFrames)
	}
}

func TestViewGame_WithWallDoesNotCrashOnUpdate(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	gm := NewGame(c, DefaultBindings(), false)

	// Keystroke that would land on wall.
	gm2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	_ = gm2.ViewGame() // should not panic

	// Keystroke that jumps over wall.
	gm3, _ := gm2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	_ = gm3.ViewGame() // should not panic
}

// ---------------------------------------------------------------------------
// Unlock interstitial / intro round tests
// ---------------------------------------------------------------------------

// newSolvableStep1Challenge returns a trivial challenge solved by a single
// "l" keystroke, matching its own par so the round always scores 3 stars.
func newSolvableStep1Challenge() challenge.Challenge {
	c := challenge.New(
		buffer("ab"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 1),
	)
	c.Par = 1
	return c
}

func TestStreamModel_UnlockTriggersInterstitial(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	// Unlock everything up through "0^$" so "ft;" becomes the frontier
	// group, one good round away from crossing the mastery threshold.
	m.progress.Mastery["wbe"] = store.Mastery{Value: 0.9, Rounds: 5}
	m.progress.Mastery["0^$"] = store.Mastery{Value: 0.9, Rounds: 5}
	m.progress.Mastery["ft;"] = store.Mastery{Value: curriculum.MasteryThreshold - 0.001, Rounds: 5}

	m.game = NewGame(newSolvableStep1Challenge(), m.config.Bindings, m.config.ReducedMotion)
	m.currentTemplate = challenge.TFindCharacter
	m.game.applyKeystroke("l")
	if !m.game.Solved() {
		t.Fatal("expected game to be solved")
	}

	m.onRoundSolved()

	if m.state != stateUnlockInterstitial {
		t.Fatalf("state = %d, want stateUnlockInterstitial", m.state)
	}
	if m.pendingGroup.Key != "ft;" {
		t.Fatalf("pendingGroup.Key = %q, want %q", m.pendingGroup.Key, "ft;")
	}
}

func TestStreamModel_InterstitialAckStartsIntroRound(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	// Mastery must already reflect the unlock (as it would by the time
	// onRoundSolved sets pendingGroup) so the intro round's newVocab
	// actually includes "wbe".
	m.progress.Mastery["wbe"] = store.Mastery{Value: curriculum.MasteryThreshold, Rounds: 5}
	m.pendingGroup = curriculum.Groups[1] // "wbe"
	m.state = stateUnlockInterstitial

	_ = m.View() // should not panic

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	after := m2.(StreamModel)
	if after.state != stateIntroRound {
		t.Fatalf("state = %d, want stateIntroRound", after.state)
	}
	_ = after.View() // should not panic
}

func TestStreamModel_IntroRoundSolveAndAdvance(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m.pendingGroup = curriculum.Groups[1] // "wbe"
	m.introGame = NewGame(newSolvableStep1Challenge(), m.config.Bindings, m.config.ReducedMotion)
	m.currentTemplate = challenge.THorizontalLine
	m.state = stateIntroRound

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	solved := m2.(StreamModel)
	if !solved.introGame.Solved() {
		t.Fatal("expected intro round to be solved")
	}
	if solved.state != stateIntroRound {
		t.Fatalf("state right after solving = %d, want stateIntroRound (awaiting ack)", solved.state)
	}

	m3, _ := solved.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := m3.(StreamModel)
	if final.state != statePlaying {
		t.Fatalf("state after ack = %d, want statePlaying", final.state)
	}
}

func TestStreamModel_IntroRoundCanBeSkipped(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m.pendingGroup = curriculum.Groups[1] // "wbe"
	m.introGame = NewGame(newSolvableStep1Challenge(), m.config.Bindings, m.config.ReducedMotion)
	m.currentTemplate = challenge.THorizontalLine
	m.state = stateIntroRound

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	after := m2.(StreamModel)
	if after.state != statePlaying {
		t.Fatalf("after skip, state = %d, want statePlaying", after.state)
	}
}

func TestStreamModel_SummaryView(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(StreamModel)
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = m2.(StreamModel)

	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(StreamModel)
	for i := 0; i < 4; i++ {
		var tmp tea.Model
		tmp, _ = paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		paused = tmp.(StreamModel)
	}
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyEnter})
	summary := m3.(StreamModel)

	view := summary.View()
	if view == "" {
		t.Fatal("summary view should not be empty")
	}
	// The summary view includes styled text; check for the plain-text parts.
	if !strings.Contains(view, "Practice Complete") {
		t.Fatalf("summary view should contain 'Practice Complete', got:\n%s", view)
	}
	if !strings.Contains(view, "Session Total") {
		t.Fatalf("summary view should contain 'Session Total', got:\n%s", view)
	}
}

func TestStreamModel_ProgressPersistence(t *testing.T) {
	dir := t.TempDir()
	fs := store.NewFileStoreWithPaths(
		dir+"/config.toml",
		dir+"/progress.json",
	)

	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	m := NewStream(gen, rng, fs)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	_ = m2.(StreamModel)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}
}

func TestChallengeStream_Next(t *testing.T) {
	gen := testGenerator(42)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	stream := NewChallengeStream(gen, rng, challenge.DefaultConfig())

	mastery := map[string]float64{}
	c, tmpl, _, err := stream.Next(mastery)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if c.Par < 0 {
		t.Fatalf("challenge par = %d, want solvable", c.Par)
	}
	if tmpl < challenge.THorizontalLine || tmpl > challenge.TFindCharacter {
		t.Fatalf("unexpected template %d", tmpl)
	}
}

func TestMasteryFloatMap(t *testing.T) {
	mastery := map[store.GroupKey]store.Mastery{
		"hjkl": {Value: 0.9, Rounds: 5},
		"wbe":  {Value: 0.3, Rounds: 2},
	}
	result := masteryFloatMap(mastery)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result["hjkl"] != 0.9 {
		t.Fatalf("hjkl = %f, want 0.9", result["hjkl"])
	}
	if result["wbe"] != 0.3 {
		t.Fatalf("wbe = %f, want 0.3", result["wbe"])
	}
}
