// Package vim provides a pure step function for vim motions.
//
// It has no dependencies on Bubble Tea or any TUI library — correctness
// is exhaustively testable in table tests (ADR 0004).
package vim

// Buffer holds the textual content of the editor.
type Buffer struct {
	Lines []string
}

// Cursor represents a position within a Buffer.
type Cursor struct {
	Row, Col int
}

// State is the complete interpreter state at one instant.
type State struct {
	Buffer Buffer
	Cursor Cursor
}

// Step applies a single keystroke to the given State and returns the
// resulting State.
//
// Currently supported keystrokes:
//   - "h": move cursor left one column, clamped to column 0
//   - "l": move cursor right one column, clamped to end of line
//
// Unrecognised keystrokes are silently ignored.
func Step(s State, keystroke string) State {
	switch keystroke {
	case "h":
		if s.Cursor.Col > 0 {
			s.Cursor.Col--
		}
	case "l":
		line := s.Buffer.Lines[s.Cursor.Row]
		if s.Cursor.Col < len(line)-1 {
			s.Cursor.Col++
		}
	}
	return s
}
