// Package solver finds the optimal (minimal-keystroke) solution for a
// challenge via breadth-first search over vim.Step, restricted to a
// supplied vocabulary (ADR 0004).
package solver

import (
	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/vim"
)

// DefaultMaxDepth is the maximum BFS depth for typical MVP-sized buffers.
const DefaultMaxDepth = 200

// stateKey uniquely identifies a vim.State for visited-state tracking,
// excluding the Buffer which is immutable during a solve.
//
// All fields except Buffer contribute to the key because Buffer never
// changes during a single solve run.
type stateKey struct {
	row, col        int
	desiredCol      int
	pending         string
	lastFTCommand   string
	lastFTChar      rune
}

// toKey converts a vim.State to a stateKey for hashing.
func toKey(s vim.State) stateKey {
	return stateKey{
		row:           s.Cursor.Row,
		col:           s.Cursor.Col,
		desiredCol:    s.DesiredCol,
		pending:       s.Pending,
		lastFTCommand: s.LastFT.Command,
		lastFTChar:    s.LastFT.Char,
	}
}

// Solve runs a breadth-first search over the vocabulary to find the
// minimal number of keystrokes that satisfy the challenge's goal
// predicate. If no solution exists within maxDepth it returns -1.
func Solve(c challenge.Challenge, vocabulary []string, maxDepth int) int {
	initial := vim.State{
		Buffer:     c.InitialBuffer,
		Cursor:     c.InitialCursor,
		DesiredCol: -1,
	}
	return OptimalFromState(initial, c, vocabulary, maxDepth)
}

// OptimalFromState returns the minimal keystrokes from the given state
// to the goal. Returns -1 if unsolvable within maxDepth.
func OptimalFromState(st vim.State, c challenge.Challenge, vocabulary []string, maxDepth int) int {
	if c.Goal(st) {
		return 0
	}
	// Can't start on a wall.
	if c.IsWall(st.Cursor.Row, st.Cursor.Col) {
		return -1
	}

	visited := map[stateKey]bool{toKey(st): true}
	queue := []vim.State{st}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		depth++
		next := make([]vim.State, 0, len(queue)*len(vocabulary))

		for _, s := range queue {
			for _, k := range vocabulary {
				ns := vim.Step(s, k)
				key := toKey(ns)
				if visited[key] {
					continue
				}
				visited[key] = true
				// Wall cells are unreachable.
				if c.IsWall(ns.Cursor.Row, ns.Cursor.Col) {
					continue
				}
				if c.Goal(ns) {
					return depth
				}
				next = append(next, ns)
			}
		}
		queue = next
	}

	return -1
}

// bfsNode tracks a state and the first keystroke from the origin state.
type bfsNode struct {
	state    vim.State
	firstKey string // first keystroke from the original starting state
}

// FirstStepFromState returns the first keystroke of an optimal solution
// from the given state, and the total optimal distance. Returns ("", -1)
// if unsolvable within maxDepth, or ("", 0) if already at the goal.
func FirstStepFromState(st vim.State, c challenge.Challenge, vocabulary []string, maxDepth int) (string, int) {
	if c.Goal(st) {
		return "", 0
	}
	// Can't start on a wall.
	if c.IsWall(st.Cursor.Row, st.Cursor.Col) {
		return "", -1
	}

	visited := map[stateKey]bool{toKey(st): true}
	queue := []bfsNode{{state: st, firstKey: ""}}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		depth++
		next := make([]bfsNode, 0, len(queue)*len(vocabulary))

		for _, n := range queue {
			for _, k := range vocabulary {
				ns := vim.Step(n.state, k)
				key := toKey(ns)
				if visited[key] {
					continue
				}
				visited[key] = true

				// Wall cells are unreachable.
				if c.IsWall(ns.Cursor.Row, ns.Cursor.Col) {
					continue
				}

				// Determine the first keystroke from the original start.
				fk := k
				if n.firstKey != "" {
					fk = n.firstKey
				}

				if c.Goal(ns) {
					return fk, depth
				}
				next = append(next, bfsNode{state: ns, firstKey: fk})
			}
		}
		queue = next
	}

	return "", -1
}