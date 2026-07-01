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
	s := New(c)

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
	s := New(c)

	if !s.Solved() {
		t.Fatal("session should be solved immediately")
	}

	// Keystrokes after solved should be ignored.
	s.Step("l")
	if got := s.State().Cursor.Col; got != 2 {
		t.Fatalf("cursor should not move after solved, got col=%d", got)
	}
}
