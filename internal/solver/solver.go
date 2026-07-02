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
		Buffer:      c.InitialBuffer,
		Cursor:      c.InitialCursor,
		DesiredCol:  -1,
	}

	if c.Goal(initial) {
		return 0
	}

	visited := map[stateKey]bool{toKey(initial): true}
	queue := []vim.State{initial}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		depth++
		next := make([]vim.State, 0, len(queue)*len(vocabulary))

		for _, st := range queue {
			for _, k := range vocabulary {
				ns := vim.Step(st, k)
				key := toKey(ns)
				if visited[key] {
					continue
				}
				visited[key] = true
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