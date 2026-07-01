// Package session orchestrates one attempt at a challenge (a round of play).
//
// It feeds keystrokes through the vim step function and checks the goal
// predicate after each step. It has no dependency on Bubble Tea or any
// TUI library.
package session

import (
	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/vim"
)

// Session models one attempt at a Challenge.
type Session struct {
	state   vim.State
	goal    challenge.GoalPredicate
	solved  bool
}

// New creates a Session for the given Challenge, initialised to the
// challenge's starting state.
func New(c challenge.Challenge) *Session {
	st := vim.State{Buffer: c.InitialBuffer, Cursor: c.InitialCursor}
	solved := c.Goal(st)
	return &Session{
		state:  st,
		goal:   c.Goal,
		solved: solved,
	}
}

// Step applies a single keystroke and checks the goal predicate.
// If the session is already solved, the keystroke is ignored.
func (s *Session) Step(keystroke string) {
	if s.solved {
		return
	}
	s.state = vim.Step(s.state, keystroke)
	if s.goal(s.state) {
		s.solved = true
	}
}

// State returns the current interpreter state.
func (s *Session) State() vim.State {
	return s.state
}

// Solved returns true if the goal predicate has been satisfied.
func (s *Session) Solved() bool {
	return s.solved
}
