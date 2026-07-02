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

func TestOptimalFromState_CustomState(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 1}, DesiredCol: -1}
	got := OptimalFromState(st, c, []string{"l"}, 100)
	if got != 1 {
		t.Fatalf("OptimalFromState = %d, want 1", got)
	}
}

func TestOptimalFromState_AlreadyAtGoal(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 2}, DesiredCol: -1}
	got := OptimalFromState(st, c, []string{"l"}, 100)
	if got != 0 {
		t.Fatalf("OptimalFromState at goal = %d, want 0", got)
	}
}

func TestFirstStepFromState_Simple(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 0}, DesiredCol: -1}
	key, dist := FirstStepFromState(st, c, []string{"l"}, 100)
	if key != "l" {
		t.Fatalf("FirstStepFromState key = %q, want %q", key, "l")
	}
	if dist != 2 {
		t.Fatalf("FirstStepFromState distance = %d, want 2", dist)
	}
}

func TestFirstStepFromState_AlreadyAtGoal(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)

	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 2}, DesiredCol: -1}
	key, dist := FirstStepFromState(st, c, []string{"l"}, 100)
	if key != "" {
		t.Fatalf("FirstStepFromState at goal key = %q, want empty", key)
	}
	if dist != 0 {
		t.Fatalf("FirstStepFromState at goal distance = %d, want 0", dist)
	}
}

func TestFirstStepFromState_Unsolvable(t *testing.T) {
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 10),
	)

	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 0}, DesiredCol: -1}
	key, dist := FirstStepFromState(st, c, []string{"l"}, 10)
	if key != "" {
		t.Fatalf("FirstStepFromState unsolvable key = %q, want empty", key)
	}
	if dist != -1 {
		t.Fatalf("FirstStepFromState unsolvable distance = %d, want -1", dist)
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

func TestSolve_RoutesAroundWall(t *testing.T) {
	// Buffer "abcd", start (0,0), target (0,3). Wall at (0,2).
	// With only "l" vocabulary, can't reach target because col 2 is blocked.
	wallC := challenge.NewWithWalls(
		buffer("abcd"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 3),
		challenge.WallSet{vim.Cursor{Row: 0, Col: 2}: true},
	)
	got := Solve(wallC, []string{"l"}, 100)
	if got != -1 {
		t.Fatalf("Solve with wall blocking = %d, want -1 (unsolvable via l alone)", got)
	}
}

func TestSolve_RoutesAroundWallMultipleLines(t *testing.T) {
	// Two-line buffer:
	//   line 0: "abc"
	//   line 1: "def"
	// Start (0,0), target (1,2). Wall at (0,1) and (0,2).
	// Player must go down to row 1 early, then right.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
		vim.Cursor{Row: 0, Col: 2}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc", "def"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(1, 2),
		walls,
	)
	// With "l", "j" — should find path: j (row 1), l (col 1), l (col 2) = 3 steps.
	got := Solve(c, []string{"l", "j"}, 100)
	if got != 3 {
		t.Fatalf("Solve around walls = %d, want 3 (jll)", got)
	}
}

func TestSolve_WallBlocksTarget(t *testing.T) {
	// Target cell is a wall — unsolvable via any character motion.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 2}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)
	got := Solve(c, []string{"l"}, 100)
	if got != -1 {
		t.Fatalf("Solve with target on wall = %d, want -1 (unsolvable)", got)
	}
}

func TestSolve_JumpMotionClearsWall(t *testing.T) {
	// Walls at cols 1-4, but target at col 6 (start of "world").
	// 'w' should jump from col 0 to col 6, landing clear.
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
		vim.Cursor{Row: 0, Col: 2}: true,
		vim.Cursor{Row: 0, Col: 3}: true,
		vim.Cursor{Row: 0, Col: 4}: true,
	}
	c := challenge.NewWithWalls(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
		walls,
	)
	got := Solve(c, []string{"w", "l"}, 100)
	if got != 1 {
		t.Fatalf("Solve with jump-over-wall = %d, want 1 (w)", got)
	}
}

func TestSolve_NoWallNoProblem(t *testing.T) {
	// Same challenge without walls should solve normally.
	c := challenge.New(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
	)
	got := Solve(c, []string{"l"}, 100)
	if got != 2 {
		t.Fatalf("Solve without walls = %d, want 2", got)
	}
}

func TestOptimalFromState_RoutesAroundWall(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
	}
	c := challenge.NewWithWalls(
		buffer("abc"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 2),
		walls,
	)

	// From col 0, moving via 'l' to col 1 is walled. Only 'l' available -> unsolvable.
	st := vim.State{Buffer: buffer("abc"), Cursor: vim.Cursor{Row: 0, Col: 0}, DesiredCol: -1}
	got := OptimalFromState(st, c, []string{"l"}, 100)
	if got != -1 {
		t.Fatalf("OptimalFromState past wall = %d, want -1", got)
	}
}

func TestFirstStepFromState_SuggestsJumpOverWall(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
		vim.Cursor{Row: 0, Col: 2}: true,
		vim.Cursor{Row: 0, Col: 3}: true,
		vim.Cursor{Row: 0, Col: 4}: true,
	}
	c := challenge.NewWithWalls(
		buffer("hello world"),
		vim.Cursor{Row: 0, Col: 0},
		challenge.CursorAtTarget(0, 6),
		walls,
	)
	st := vim.State{Buffer: buffer("hello world"), Cursor: vim.Cursor{Row: 0, Col: 0}, DesiredCol: -1}
	key, dist := FirstStepFromState(st, c, []string{"w", "l"}, 100)
	if key != "w" {
		t.Fatalf("FirstStepFromState past wall key = %q, want %q", key, "w")
	}
	if dist != 1 {
		t.Fatalf("FirstStepFromState past wall distance = %d, want 1", dist)
	}
}
