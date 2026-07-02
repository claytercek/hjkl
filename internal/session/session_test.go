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