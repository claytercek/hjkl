package challenge

import (
	"testing"

	"github.com/clay/hjkl/internal/vim"
)

func TestCursorAtTarget(t *testing.T) {
	p := CursorAtTarget(0, 3)

	if p(vim.State{Buffer: vim.Buffer{}, Cursor: vim.Cursor{Row: 0, Col: 2}}) {
		t.Error("predicate should be false for wrong column")
	}
	if p(vim.State{Buffer: vim.Buffer{}, Cursor: vim.Cursor{Row: 1, Col: 3}}) {
		t.Error("predicate should be false for wrong row")
	}
	if !p(vim.State{Buffer: vim.Buffer{}, Cursor: vim.Cursor{Row: 0, Col: 3}}) {
		t.Error("predicate should be true for matching position")
	}
}
