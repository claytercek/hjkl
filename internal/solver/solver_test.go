package solver

import (
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/vim"
)

// buffer is a shorthand for creating a Buffer with the given lines.
func buffer(lines ...string) vim.Buffer {
	return vim.Buffer{Lines: lines}
}

func TestSolve_AlreadyAtTarget(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 2},
		challenge.CursorAtTarget(0, 2),
	)

	got := Solve(c, []string{"l", "h"}, 100)
	if got != 0 {
		t.Fatalf("Solve = %d, want 0 (already at target)", got)
	}
}

func TestSolve_SimpleRight(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	// Vocabulary only contains "l" — must go ll.
	got := Solve(c, []string{"l"}, 100)
	if got != 2 {
		t.Fatalf("Solve = %d, want 2 (ll)", got)
	}
}

func TestSolve_WordMotion(t *testing.T) {
	c := challenge.New(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6), // start of "world"
	)

	got := Solve(c, []string{"w"}, 100)
	if got != 1 {
		t.Fatalf("Solve = %d, want 1 (w)", got)
	}
}

func TestSolve_MultiLine(t *testing.T) {
	c := challenge.New(
		buffer("abc", "def", "ghi"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(2, 1),
	)

	// j j l
	got := Solve(c, []string{"j", "l"}, 100)
	if got != 3 {
		t.Fatalf("Solve = %d, want 3 (jjl)", got)
	}
}

func TestSolve_TieDifferentKeystrokes(t *testing.T) {
	// "hello world" -> "world" can be reached by "w" (1 step) or
	// "llllll" (6 steps). Both solve but the solver finds the minimum.
	c := challenge.New(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
	)

	got := Solve(c, []string{"w", "l"}, 100)
	if got != 1 {
		t.Fatalf("Solve = %d, want 1 (w beats llllll)", got)
	}
}

func TestSolve_RestrictedVocabulary(t *testing.T) {
	// Buffer "abc def ghi", start (0,0), target (0,8).
	// With "w" and "l", optimal is "w" (col 4 = start of "def") then "w"
	// (col 8 = start of "ghi") = 2 steps.
	c := challenge.New(
		buffer("abc def ghi"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 8),
	)

	// Restricted to only "l" -> takes 8 steps
	got := Solve(c, []string{"l"}, 100)
	if got != 8 {
		t.Fatalf("Solve with only l = %d, want 8", got)
	}
}

func TestSolve_Unsolvable(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 10), // impossible, line is shorter
	)

	got := Solve(c, []string{"l"}, 10) // maxDepth limits search
	if got != -1 {
		t.Fatalf("Solve = %d, want -1 (unsolvable)", got)
	}
}

func TestSolve_MaxDepthBound(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	// maxDepth=1 should not find the solution (needs 2 steps).
	got := Solve(c, []string{"l"}, 1)
	if got != -1 {
		t.Fatalf("Solve with maxDepth=1 = %d, want -1", got)
	}
}

func TestSolve_NoPendingLeakBetweenLevels(t *testing.T) {
	// Start on "hello world" at col 0, target "o" at col 4.
	// "fo" (f then o) should find it in 2 steps.
	c := challenge.New(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 4),
	)

	got := Solve(c, []string{"f", "o"}, 100)
	if got != 2 {
		t.Fatalf("Solve = %d, want 2 (fo)", got)
	}
}

func TestSolve_VisitedStateDoesNotRevisit(t *testing.T) {
	// Verifies that visited-state tracking prevents exponential blowup
	// on a simple challenge where many paths lead to the same state.
	// "abc" start (0,0) target (0,2) — "hl" and "lh" both reach col 0 then col 1.
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	got := Solve(c, []string{"l", "h"}, 100)
	if got != 2 {
		t.Fatalf("Solve = %d, want 2 (ll)", got)
	}
}
