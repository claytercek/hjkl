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

// StarsFor computes the star rating given the actual keystroke count
// and optimal par.
//
//	3 stars — at or under par
//	2 stars — par+1 to par+2
//	1 star  — completed (more than par+2)
func StarsFor(keystrokes, par int) int {
	switch {
	case par < 0:
		// No par available (solver didn't find a solution).
		return 1
	case keystrokes <= par:
		return 3
	case keystrokes <= par+2:
		return 2
	default:
		return 1
	}
}

// Result holds the outcome of a completed session.
type Result struct {
	Keystrokes int
	Par        int // -1 if no optimal solution was found
	Stars      int
}

// Session models one attempt at a Challenge.
type Session struct {
	state          vim.State
	goal           challenge.GoalPredicate
	solved         bool
	keystrokeCount int
	par            int // -1 = not computed / no solution
}

// New creates a Session for the given Challenge, initialised to the
// challenge's starting state. If par >= 0 it is used for star-band
// display; pass -1 when par is unknown.
func New(c challenge.Challenge, par int) *Session {
	st := vim.State{Buffer: c.InitialBuffer, Cursor: c.InitialCursor}
	return &Session{
		state:          st,
		goal:           c.Goal,
		solved:         c.Goal(st),
		keystrokeCount: 0,
		par:            par,
	}
}

// Step applies a single keystroke and checks the goal predicate.
// If the session is already solved, the keystroke is ignored.
func (s *Session) Step(keystroke string) {
	if s.solved {
		return
	}
	s.keystrokeCount++
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

// KeystrokeCount returns the number of keystrokes applied so far.
func (s *Session) KeystrokeCount() int {
	return s.keystrokeCount
}

// Result returns the outcome of the session. It is only meaningful
// after Solved returns true.
func (s *Session) Result() Result {
	return Result{
		Keystrokes: s.keystrokeCount,
		Par:        s.par,
		Stars:      StarsFor(s.keystrokeCount, s.par),
	}
}