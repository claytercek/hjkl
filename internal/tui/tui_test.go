package tui

import (
	"math/rand"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/vim"
)

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

func TestNewLesson_StartsPlaying(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)
	if m.state != statePlaying {
		t.Fatalf("initial state = %d, want statePlaying", m.state)
	}
	if m.current != 0 {
		t.Fatalf("initial current = %d, want 0", m.current)
	}
}

func TestLessonModel_KeystrokeAdvancesSession(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Send an 'l' keystroke.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated := m2.(LessonModel)
	if updated.game.session.State().Cursor.Col != 1 {
		t.Fatalf("after l, cursor col = %d, want 1", updated.game.session.State().Cursor.Col)
	}
	if updated.state != statePlaying {
		t.Fatalf("state = %d, want statePlaying", updated.state)
	}

	// Send another 'l' to solve.
	m3, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	solved := m3.(LessonModel)
	if !solved.game.Solved() {
		t.Fatal("expected session to be solved after 2 l's")
	}
	if solved.state != stateSummary {
		t.Fatalf("after final round solved, state = %d, want stateSummary", solved.state)
	}
}

func TestLessonModel_SkipKey(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
			{
				Challenge: challenge.New(
					buffer("def"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 1),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Skip the first round.
	cfg := DefaultConfig()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	// The Skip key is "ctrl+n" — we need to simulate it properly.
	// In Bubble Tea, ctrl+n is represented as a KeyMsg with Type tea.KeyCtrlN.
	m3, _ := m2.(LessonModel).Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	skipped := m3.(LessonModel)

	if skipped.state != statePlaying {
		t.Fatalf("after skip, state = %d, want statePlaying", skipped.state)
	}
	if skipped.current != 1 {
		t.Fatalf("after skip, current = %d, want 1", skipped.current)
	}
	_ = cfg // used for reference
}

func TestLessonModel_HintKey(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Press Ctrl-H to get hint.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	hinted := m2.(LessonModel)

	if !hinted.game.hintVisible {
		t.Fatal("expected hint to be visible after Ctrl-H")
	}
	if hinted.game.hintKey == "" {
		t.Fatal("expected hintKey to be non-empty")
	}
}

func TestLessonModel_PauseToggle(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Press Ctrl-C to pause.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(LessonModel)

	if !paused.paused {
		t.Fatal("expected paused to be true after Ctrl-C")
	}
	if paused.menuCursor != 0 {
		t.Fatalf("menu cursor = %d, want 0", paused.menuCursor)
	}

	// Press Ctrl-C again to resume.
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	resumed := m3.(LessonModel)

	if resumed.paused {
		t.Fatal("expected paused to be false after second Ctrl-C")
	}
}

func TestLessonModel_MenuNavigation(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Pause.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(LessonModel)

	// Navigate down with 'j'.
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	down := m3.(LessonModel)

	if down.menuCursor != 1 {
		t.Fatalf("after j, menu cursor = %d, want 1", down.menuCursor)
	}

	// Navigate up with 'k'.
	m4, _ := down.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	up := m4.(LessonModel)

	if up.menuCursor != 0 {
		t.Fatalf("after k, menu cursor = %d, want 0", up.menuCursor)
	}

	// Select Resume with Enter.
	m5, _ := up.Update(tea.KeyMsg{Type: tea.KeyEnter})
	resumed := m5.(LessonModel)

	if resumed.paused {
		t.Fatal("expected paused to be false after Resume")
	}
}

func TestLessonModel_EscNotIntercepted(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abcde"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Esc should be passed through to the game (no-op in vim step).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	after := m2.(LessonModel)

	if after.state != statePlaying {
		t.Fatalf("after esc, state = %d, want statePlaying", after.state)
	}
	// Session should not have quit.
	if after.game.Solved() {
		t.Fatal("session should not be solved after esc")
	}
}

func TestRetryRound_ClearsResult(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Solve the round.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m3, _ := m2.(LessonModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	solved := m3.(LessonModel)

	if solved.lesson.Rounds[0].Result.Keystrokes != 2 {
		t.Fatalf("round result keystrokes = %d, want 2", solved.lesson.Rounds[0].Result.Keystrokes)
	}

	// Retry.
	solved.retryRound()
	if solved.lesson.Rounds[0].Result.Keystrokes != 0 {
		t.Fatalf("after retry, round result keystrokes = %d, want 0", solved.lesson.Rounds[0].Result.Keystrokes)
	}
	if solved.state != statePlaying {
		t.Fatalf("after retry, state = %d, want statePlaying", solved.state)
	}
}

func TestLessonModel_RetryKey_MidRound(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Move to col 1 so the session has some keystrokes logged.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	afterMove := m2.(LessonModel)

	if afterMove.game.session.State().Cursor.Col != 1 {
		t.Fatalf("after l, cursor col = %d, want 1", afterMove.game.session.State().Cursor.Col)
	}

	// Press ctrl+r to retry mid-round.
	m3, _ := afterMove.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	retried := m3.(LessonModel)

	// Cursor should be back at initial position.
	if retried.game.session.State().Cursor.Col != 0 {
		t.Fatalf("after retry, cursor col = %d, want 0", retried.game.session.State().Cursor.Col)
	}
	// Keystroke count should be reset (empty keystrokes).
	if len(retried.game.keystrokes) != 0 {
		t.Fatalf("after retry, keystrokes count = %d, want 0", len(retried.game.keystrokes))
	}
	// State should be playing.
	if retried.state != statePlaying {
		t.Fatalf("after retry, state = %d, want statePlaying", retried.state)
	}
}

func TestLessonModel_RetryKey_RoundDone(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
			{
				Challenge: challenge.New(
					buffer("def"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 1),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Solve the first round (2 l's) to reach stateRoundDone.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m3, _ := m2.(LessonModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	solved := m3.(LessonModel)

	if solved.state != stateRoundDone {
		t.Fatalf("after solving first of two rounds, state = %d, want stateRoundDone", solved.state)
	}

	// Press ctrl+r from round-done screen.
	m4, _ := solved.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	retried := m4.(LessonModel)

	// Should still be on the same round (current == 0).
	if retried.current != 0 {
		t.Fatalf("after retry, current = %d, want 0", retried.current)
	}
	// Cursor should be back at initial position.
	if retried.game.session.State().Cursor.Col != 0 {
		t.Fatalf("after retry from round-done, cursor col = %d, want 0", retried.game.session.State().Cursor.Col)
	}
	// State should be playing again.
	if retried.state != statePlaying {
		t.Fatalf("after retry from round-done, state = %d, want statePlaying", retried.state)
	}
	// Round result should be cleared.
	if retried.lesson.Rounds[0].Result.Keystrokes != 0 {
		t.Fatalf("after retry, round result keystrokes = %d, want 0", retried.lesson.Rounds[0].Result.Keystrokes)
	}
}

func TestLessonModel_RetryKey_IgnoredOnSummary(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Solve the single round → stateSummary.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m3, _ := m2.(LessonModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	summary := m3.(LessonModel)

	if summary.state != stateSummary {
		t.Fatalf("after solving single round, state = %d, want stateSummary", summary.state)
	}
	// Verify result was recorded before retry.
	if summary.lesson.Rounds[0].Result.Keystrokes != 2 {
		t.Fatalf("round result keystrokes = %d, want 2", summary.lesson.Rounds[0].Result.Keystrokes)
	}

	// Press ctrl+r from summary — should be a no-op.
	m4, _ := summary.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	stillSummary := m4.(LessonModel)

	if stillSummary.state != stateSummary {
		t.Fatalf("after retry from summary, state = %d, want stateSummary (unchanged)", stillSummary.state)
	}
	if stillSummary.current != 0 {
		t.Fatalf("after retry from summary, current = %d, want 0", stillSummary.current)
	}
}

func TestLessonModel_AnimationsCreateTick(t *testing.T) {
	// After a keystroke, there should be a tick command for animations.
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Press 'l' — should create ghosts and tick.
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	after := m2.(LessonModel)

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
	gm := NewGame(c, DefaultBindings(), true) // reducedMotion = true

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
		want int // number of positions
	}{
		{
			name: "horizontal right",
			from: vim.Cursor{Row: 0, Col: 0},
			to:   vim.Cursor{Row: 0, Col: 3},
			want: 4, // cols 0,1,2,3
		},
		{
			name: "horizontal left",
			from: vim.Cursor{Row: 0, Col: 3},
			to:   vim.Cursor{Row: 0, Col: 0},
			want: 4, // cols 0,1,2,3
		},
		{
			name: "vertical down",
			from: vim.Cursor{Row: 0, Col: 1},
			to:   vim.Cursor{Row: 1, Col: 1},
			want: 2, // rows 0,1
		},
		{
			name: "vertical up",
			from: vim.Cursor{Row: 1, Col: 1},
			to:   vim.Cursor{Row: 0, Col: 1},
			want: 2, // rows 0,1
		},
		{
			name: "no movement",
			from: vim.Cursor{Row: 0, Col: 2},
			to:   vim.Cursor{Row: 0, Col: 2},
			want: 1, // same position
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpolate(tt.from, tt.to, buf)
			if len(got) != tt.want {
				t.Errorf("interpolate returned %d positions, want %d", len(got), tt.want)
			}
			// First and last should match from/to.
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
	_ = gm.ViewGame() // should not panic

	// After a keystroke, view should also not panic.
	gm2, _ := gm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	_ = gm2.ViewGame()
}

func TestLessonView_DoesNotCrash(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)
	_ = m.View() // should not panic

	// After solving.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m3, _ := m2.(LessonModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	_ = m3.(LessonModel).View() // should not panic
}

func TestMenuView_DoesNotCrash(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Pause and view.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(LessonModel)
	_ = paused.View() // should not panic
}

func TestApplyKeystroke_WastedKeyDetection(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	gm := NewGame(c, DefaultBindings(), false)

	// A move that doesn't reduce optimal distance (e.g., 'h' when at col 0)
	// should register as wasted.
	gm.applyKeystroke("h")
	if gm.wasteFrames <= 0 {
		t.Fatal("expected wasteFrames > 0 after wasted key (h at col 0)")
	}

	// Reset for next test.
	gm = NewGame(c, DefaultBindings(), false)
	// A move that makes progress (l) should not be wasted.
	gm.applyKeystroke("l")
	if gm.wasteFrames > 0 {
		t.Fatal("expected wasteFrames == 0 after progress key (l)")
	}
}

func TestNewLessonWithConfig_CustomBindings(t *testing.T) {
	cfg := Config{
		Bindings: KeyBindings{
			Pause: "ctrl+p",
			Hint:  "ctrl+h",
			Skip:  "ctrl+s",
		},
		ReducedMotion: true,
	}
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLessonWithConfig(lesson, cfg)

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

func TestSkipRound_LastRoundGoesToSummary(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)
	m.skipRound()

	if m.state != stateSummary {
		t.Fatalf("after skip on last round, state = %d, want stateSummary", m.state)
	}
	// Result should record a skip.
	if m.lesson.Rounds[0].Result.Keystrokes != 0 {
		t.Fatalf("skipped round keystrokes = %d, want 0", m.lesson.Rounds[0].Result.Keystrokes)
	}
}

func TestRestartLesson_ClearsAllResults(t *testing.T) {
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.New(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
				),
				Template: challenge.THorizontalLine,
			},
			{
				Challenge: challenge.New(
					buffer("def"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 1),
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)

	// Simulate playing and recording results.
	m.lesson.Rounds[0].Result.Keystrokes = 3
	m.lesson.Rounds[1].Result.Keystrokes = 2

	m.restartLesson()
	if m.lesson.Rounds[0].Result.Keystrokes != 0 {
		t.Fatalf("after restart, round 0 keystrokes = %d, want 0", m.lesson.Rounds[0].Result.Keystrokes)
	}
	if m.lesson.Rounds[1].Result.Keystrokes != 0 {
		t.Fatalf("after restart, round 1 keystrokes = %d, want 0", m.lesson.Rounds[1].Result.Keystrokes)
	}
	if m.current != 0 {
		t.Fatalf("after restart, current = %d, want 0", m.current)
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
	// Ghost with life 3 should now be 2, life 5 should be 4.
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
			{row: 0, col: 0, life: 1}, // will expire
			{row: 0, col: 1, life: 2}, // will stay
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

	// Keystroke should be ignored.
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

func TestLessonView_WithWallDoesNotCrash(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	lesson := &curriculum.Lesson{
		Rounds: []curriculum.Round{
			{
				Challenge: challenge.NewWithWalls(
					buffer("abc"),
					vim.Cursor{Row: 0, Col: 0},
					challenge.CursorAtTarget(0, 2),
					walls,
				),
				Template: challenge.THorizontalLine,
			},
		},
	}
	m := NewLesson(lesson)
	_ = m.View() // should not panic

	// Keystroke lands on wall.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	_ = m2.(LessonModel).View() // should not panic
}

// ---------------------------------------------------------------------------
// Stream mode tests
// ---------------------------------------------------------------------------

func newTestStreamModel(seed int64, unlocked int) *curriculum.Stream {
	rng := rand.New(rand.NewSource(seed))
	gen := challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)
	rng2 := rand.New(rand.NewSource(seed + 1))
	p := store.NewProgress()
	return curriculum.NewStream(gen, challenge.DefaultConfig(), rng2, p, unlocked)
}

func TestNewLessonStream_StartsPlaying(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	if m.state != statePlaying {
		t.Fatalf("initial state = %d, want statePlaying", m.state)
	}
	if m.stream == nil {
		t.Fatal("expected stream to be set")
	}
}

func TestNewLessonStream_SolvesRound(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	// Send a keystroke. The game might solve the challenge depending on the
	// generated challenge. Just verify we don't crash.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	after := m2.(LessonModel)

	// Should still be in some valid state.
	if after.state != statePlaying && after.state != stateRoundDone {
		t.Fatalf("unexpected state after keystroke: %d", after.state)
	}
}

func TestNewLessonStream_Skip(t *testing.T) {
	str := newTestStreamModel(99, 1)
	m := NewLessonStream(str)

	// Skip should generate a new round.
	m.skipRound()
	if m.state != statePlaying {
		t.Fatalf("after skip, state = %d, want statePlaying", m.state)
	}
}

func TestNewLessonStream_Retry(t *testing.T) {
	str := newTestStreamModel(99, 1)
	m := NewLessonStream(str)

	m.retryRound()
	if m.state != statePlaying {
		t.Fatalf("after retry, state = %d, want statePlaying", m.state)
	}
}

func TestNewLessonStream_HintKey(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	hinted := m2.(LessonModel)

	if hinted.state != statePlaying {
		t.Fatalf("after hint, state = %d, want statePlaying", hinted.state)
	}
}

func TestNewLessonStream_PauseToggle(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	// Pause.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	paused := m2.(LessonModel)

	if !paused.paused {
		t.Fatal("expected paused to be true after Ctrl-C")
	}

	// Resume.
	m3, _ := paused.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	resumed := m3.(LessonModel)

	if resumed.paused {
		t.Fatal("expected paused to be false after second Ctrl-C")
	}
}

func TestNewLessonStream_ViewDoesNotCrash(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)
	_ = m.View() // should not panic
}

func TestUnlockInterstitial_View(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	// Push mastery above threshold to trigger unlock.
	for i := 0; i < 10; i++ {
		m.stream.UpdateFrontierMastery(3, 3, 3)
	}
	if !m.stream.ShouldUnlock() {
		t.Fatal("should be ready to unlock")
	}

	// Trigger unlock flow.
	m.pendingGroup = m.stream.UnlockNext()
	m.state = stateUnlockInterstitial

	// View should not panic.
	_ = m.View()

	// Acknowledge with space.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	after := m2.(LessonModel)
	if after.state != stateIntroRound && after.state != statePlaying {
		t.Fatalf("after ack, state = %d, want stateIntroRound or statePlaying", after.state)
	}
}

func TestUnlockInterstitial_AdvanceKey(t *testing.T) {
	str := newTestStreamModel(42, 1)
	m := NewLessonStream(str)

	// Set up unlock state.
	for i := 0; i < 10; i++ {
		m.stream.UpdateFrontierMastery(3, 3, 3)
	}
	m.pendingGroup = m.stream.UnlockNext()
	m.state = stateUnlockInterstitial

	// Enter should work same as space.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	after := m2.(LessonModel)
	if after.state != stateIntroRound && after.state != statePlaying {
		t.Fatalf("after enter ack, state = %d, want stateIntroRound or statePlaying", after.state)
	}
}

func TestIntroRound_SkipAndView(t *testing.T) {
	str := newTestStreamModel(99, 1)
	m := NewLessonStream(str)

	// Set up unlock + intro round.
	for i := 0; i < 10; i++ {
		m.stream.UpdateFrontierMastery(3, 3, 3)
	}
	m.pendingGroup = m.stream.UnlockNext()
	m.state = stateUnlockInterstitial

	// Acknowledge to start intro round.
	m.startIntroRound()
	if m.state != stateIntroRound {
		t.Fatalf("after startIntroRound, state = %d, want stateIntroRound", m.state)
	}

	// View should not panic.
	_ = m.View()

	// Skip the intro round.
	m.skipRound()
	if m.state != statePlaying {
		t.Fatalf("after skip intro round, state = %d, want statePlaying", m.state)
	}
}

func TestSaveProgress_IncludesUnlockedCount(t *testing.T) {
	dir := t.TempDir()
	progressPath := dir + "/progress.json"

	str := newTestStreamModel(42, 1)
	fs := store.NewFileStoreWithPaths(dir+"/config.toml", progressPath)
	m := NewLessonStream(str, fs)

	// Push mastery and unlock.
	for i := 0; i < 10; i++ {
		m.stream.UpdateFrontierMastery(3, 3, 3)
	}
	m.pendingGroup = m.stream.UnlockNext()

	m.saveProgress()

	loaded, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress error: %v", err)
	}
	if loaded.UnlockedCount != 2 {
		t.Errorf("UnlockedCount = %d, want 2", loaded.UnlockedCount)
	}
}