package session

import (
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/vim"
)

func TestSession_StepsToSolution(t *testing.T) {
	c := challenge.New(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	s := New(c, -1)

	if s.Solved() {
		t.Fatal("session should not be solved initially")
	}

	// Move right twice to reach target.
	s.Step("l")
	if s.Solved() {
		t.Fatal("session should not be solved after one step")
	}
	if got := s.State().Cursor.Col; got != 1 {
		t.Fatalf("cursor col = %d, want 1", got)
	}

	s.Step("l")
	if !s.Solved() {
		t.Fatal("session should be solved after reaching target")
	}
	if got := s.State().Cursor.Col; got != 2 {
		t.Fatalf("cursor col = %d, want 2", got)
	}
}

func TestSession_SolvedIgnoresKeystrokes(t *testing.T) {
	c := challenge.New(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 2}, // already at target
		challenge.CursorAtTarget(0, 2),
	)
	s := New(c, -1)

	if !s.Solved() {
		t.Fatal("session should be solved immediately")
	}

	// Keystrokes after solved should be ignored.
	s.Step("l")
	if got := s.State().Cursor.Col; got != 2 {
		t.Fatalf("cursor should not move after solved, got col=%d", got)
	}
}

func TestSession_KeystrokeCount(t *testing.T) {
	c := challenge.New(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2), // requires 2 'l' presses
	)
	s := New(c, -1)

	if got := s.KeystrokeCount(); got != 0 {
		t.Fatalf("initial keystroke count = %d, want 0", got)
	}

	s.Step("l")
	if got := s.KeystrokeCount(); got != 1 {
		t.Fatalf("after 1 step, keystroke count = %d, want 1", got)
	}
	if s.Solved() {
		t.Fatal("should not be solved after 1 step")
	}

	s.Step("l")
	if got := s.KeystrokeCount(); got != 2 {
		t.Fatalf("after 2 steps, keystroke count = %d, want 2", got)
	}
	if !s.Solved() {
		t.Fatal("should be solved after 2 steps")
	}
}

func TestSession_ResultBasic(t *testing.T) {
	c := challenge.New(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	s := New(c, 3) // par = 3

	s.Step("l")
	s.Step("l")

	r := s.Result()
	if r.Keystrokes != 2 {
		t.Fatalf("Keystrokes = %d, want 2", r.Keystrokes)
	}
	if r.Par != 3 {
		t.Fatalf("Par = %d, want 3", r.Par)
	}
	if r.Stars != 3 {
		t.Fatalf("Stars = %d, want 3 (at or under par)", r.Stars)
	}
}

func TestSession_StarBandScenarios(t *testing.T) {
	// Use a challenge that needs exactly 3 keystrokes so we can control
	// the count precisely by using "l" three times from a longer buffer.
	tests := []struct {
		name      string
		par       int
		wantStars int
	}{
		{"at par", 3, 3},
		{"under par", 5, 3},
		{"par+1", 2, 2},
		{"par+2", 1, 2},
		{"over par+2", 0, 1},
		{"no par (-1)", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := challenge.New(
				vim.Buffer{Lines: []string{"abcde"}},
				vim.Cursor{Row: 0, Col: 0},
				challenge.CursorAtTarget(0, 3),
			)
			s := New(c, tt.par)

			s.Step("l")
			s.Step("l")
			s.Step("l")

			if !s.Solved() {
				t.Fatal("session should be solved after 3 l's")
			}

			r := s.Result()
			if r.Keystrokes != 3 {
				t.Fatalf("Keystrokes = %d, want 3", r.Keystrokes)
			}
			if r.Stars != tt.wantStars {
				t.Errorf("Stars = %d, want %d (keystrokes=%d, par=%d)",
					r.Stars, tt.wantStars, r.Keystrokes, tt.par)
			}
		})
	}
}

func TestStarsFor(t *testing.T) {
	tests := []struct {
		k, par int
		want   int
	}{
		{1, 2, 3},
		{2, 2, 3},
		{3, 2, 2},
		{4, 2, 2},
		{5, 2, 1},
		{10, 2, 1},
		{0, 0, 3},
		{3, -1, 1}, // no par
	}

	for _, tt := range tests {
		got := StarsFor(tt.k, tt.par)
		if got != tt.want {
			t.Errorf("StarsFor(%d, %d) = %d, want %d", tt.k, tt.par, got, tt.want)
		}
	}
}

func TestSession_WallRefusesMove(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	s := New(c, 3)

	// 'l' from col 0 lands on wall at col 1 — cursor should revert.
	s.Step("l")
	if got := s.State().Cursor.Col; got != 0 {
		t.Fatalf("after wall refusal, cursor col = %d, want 0", got)
	}
	if s.KeystrokeCount() != 1 {
		t.Fatalf("keystroke count = %d, want 1 after wall refusal", s.KeystrokeCount())
	}
	if s.Solved() {
		t.Fatal("session should not be solved after wall refusal")
	}
}

func TestSession_WallRefusalKeystrokesCount(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		vim.Buffer{Lines: []string{"abcd"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 1), // target is ON the wall — unreachable by l
		walls,
	)
	s := New(c, 5)

	// Try to move right — lands on wall.
	s.Step("l")
	if got := s.State().Cursor.Col; got != 0 {
		t.Fatalf("after wall refusal, cursor col = %d, want 0", got)
	}
	if s.KeystrokeCount() != 1 {
		t.Fatalf("keystroke count = %d, want 1", s.KeystrokeCount())
	}
	if s.Solved() {
		t.Fatal("should not be solved after wall refusal")
	}

	// Multiple refusals all count.
	s.Step("l")
	s.Step("l")
	if got := s.State().Cursor.Col; got != 0 {
		t.Fatalf("after 3 wall refusals, cursor col = %d, want 0", got)
	}
	if s.KeystrokeCount() != 3 {
		t.Fatalf("keystroke count = %d, want 3", s.KeystrokeCount())
	}
}

func TestSession_WallBlocksGoalOnLanding(t *testing.T) {
	// Target cell itself is a wall — should never be reachable.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 2}: true,
	}
	c := challenge.NewWithWalls(
		vim.Buffer{Lines: []string{"abc"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	s := New(c, -1)

	s.Step("l")
	s.Step("l")
	// Second step lands on target cell, but it's a wall — revert.
	if got := s.State().Cursor.Col; got != 1 {
		t.Fatalf("after two steps toward walled target, cursor col = %d, want 1", got)
	}
	if s.Solved() {
		t.Fatal("should not be solved when target cell is a wall")
	}
}

func TestSession_JumpMotionClearsWall(t *testing.T) {
	// Walls at cols 1-4, target at col 6. 'w' should jump over walls.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
		vim.Cursor{Row: 0, Col: 2}: true,
		vim.Cursor{Row: 0, Col: 3}: true,
		vim.Cursor{Row: 0, Col: 4}: true,
	}
	c := challenge.NewWithWalls(
		vim.Buffer{Lines: []string{"hello world"}},
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
		walls,
	)
	s := New(c, 1)

	// 'w' jumps over walls directly to 'world' at col 6.
	s.Step("w")
	if got := s.State().Cursor.Col; got != 6 {
		t.Fatalf("after w past walls, cursor col = %d, want 6", got)
	}
	if !s.Solved() {
		t.Fatal("should be solved after w jumps past walls")
	}
}

func TestSession_WallDoesNotRevertDesiredCol(t *testing.T) {
	// Moving down to a wall should revert both cursor AND desired col.
	walls := challenge.WallSet{
		vim.Cursor{Row: 1, Col: 2}: true,
	}
	buf := vim.Buffer{Lines: []string{"abc", "def"}}
	c := challenge.NewWithWalls(
		buf,
		vim.Cursor{Row: 0, Col: 2},
		challenge.CursorAtTarget(1, 1),
		walls,
	)
	s := New(c, 5)

	// 'j' from row 0 col 2 would go to row 1 col 2 — which is a wall.
	// Cursor should revert to row 0 col 2, and desiredCol should revert too.
	s.Step("j")
	if got := s.State().Cursor.Row; got != 0 {
		t.Fatalf("after wall refusal, cursor row = %d, want 0", got)
	}
	if got := s.State().Cursor.Col; got != 2 {
		t.Fatalf("after wall refusal, cursor col = %d, want 2", got)
	}
	if s.State().DesiredCol != -1 {
		t.Fatalf("after wall refusal, desiredCol = %d, want -1", s.State().DesiredCol)
	}
}