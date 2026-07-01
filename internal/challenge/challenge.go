// Package challenge defines the atomic unit of practice (ADR 0002).
//
// A Challenge carries a starting buffer and cursor together with a goal
// predicate that signals when the player has succeeded. It has no
// dependency on Bubble Tea or any TUI library.
package challenge

import "github.com/clay/hjkl/internal/vim"

// GoalPredicate returns true when the given state satisfies the challenge.
type GoalPredicate func(vim.State) bool

// Challenge is the atomic unit of practice.
type Challenge struct {
	// InitialBuffer is the buffer the player starts with.
	InitialBuffer vim.Buffer

	// InitialCursor is the starting cursor position.
	InitialCursor vim.Cursor

	// Goal returns true when the player has solved the challenge.
	Goal GoalPredicate
}

// New returns a Challenge with the given starting state and goal predicate.
func New(buf vim.Buffer, cursor vim.Cursor, goal GoalPredicate) Challenge {
	return Challenge{
		InitialBuffer: buf,
		InitialCursor: cursor,
		Goal:          goal,
	}
}

// CursorAtTarget returns a GoalPredicate that is satisfied when the cursor
// is at the given position.
func CursorAtTarget(row, col int) GoalPredicate {
	return func(s vim.State) bool {
		return s.Cursor.Row == row && s.Cursor.Col == col
	}
}