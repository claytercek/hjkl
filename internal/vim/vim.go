// Package vim provides a pure step function for vim motions.
//
// It has no dependencies on Bubble Tea or any TUI library — correctness
// is exhaustively testable in table tests (ADR 0004).
package vim

import "unicode"

// Buffer holds the textual content of the editor.
type Buffer struct {
	Lines []string
}

// Cursor represents a position within a Buffer.
type Cursor struct {
	Row, Col int
}

// LastFTCommand records the most recent f/t/F/T motion for ; repeat.
type LastFTCommand struct {
	Command string // "f", "t", "F", "T"
	Char    rune   // the target character
}

// State is the complete interpreter state at one instant.
type State struct {
	Buffer Buffer
	Cursor Cursor

	// DesiredCol is the column to preserve during vertical motions.
	// -1 means "use Cursor.Col" (no vertical motion has been performed).
	DesiredCol int

	// Pending holds the prefix key for a multi-key command:
	//   "f", "t", "F", "T" — awaiting a target character
	//   "g"                — awaiting a second 'g' for gg
	//   ""                 — no pending state
	Pending string

	// LastFT stores the last f/t/F/T command so ; can repeat it.
	LastFT LastFTCommand
}

// charClass returns 0 for whitespace, 1 for word characters (letters, digits,
// underscore), 2 for other non-blank characters.
func charClass(r rune) int {
	if r == ' ' || r == '\t' {
		return 0
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return 1
	}
	return 2
}

// isWORDClass returns 0 for whitespace, 1 for any non-blank character.
func isWORDClass(r rune) int {
	if r == ' ' || r == '\t' {
		return 0
	}
	return 1
}

// Step applies a single keystroke to the given State and returns the
// resulting State.
func Step(s State, keystroke string) State {
	// If there's a pending multi-key prefix, resolve it first.
	if s.Pending != "" {
		return resolvePending(s, keystroke)
	}

	switch keystroke {
	case "h", "left":
		return stepLeft(s)
	case "j", "down":
		return stepDown(s)
	case "k", "up":
		return stepUp(s)
	case "l", "right":
		return stepRight(s)
	case "0":
		return stepZero(s)
	case "^":
		return stepFirstNonBlank(s)
	case "$":
		return stepEndOfLine(s)
	case "w":
		return stepWordForward(s)
	case "b":
		return stepWordBack(s)
	case "e":
		return stepWordEnd(s)
	case "W":
		return stepWORDForward(s)
	case "B":
		return stepWORDBack(s)
	case "E":
		return stepWORDEnd(s)
	case "f", "t", "F", "T":
		s.Pending = keystroke
		return s
	case ";":
		return repeatLastFT(s)
	case "g":
		s.Pending = "g"
		return s
	case "G":
		return gotoLastLine(s)
	}
	// Unrecognised keystrokes are silently ignored.
	return s
}

// resolvePending handles a keystroke when a multi-key prefix is pending.
func resolvePending(s State, keystroke string) State {
	pending := s.Pending
	s.Pending = ""

	switch pending {
	case "f", "t", "F", "T":
		if len(keystroke) != 1 {
			// Non-character keystroke cancels the pending command.
			return s
		}
		ch := rune(keystroke[0])
		s.LastFT = LastFTCommand{Command: pending, Char: ch}
		return applyFT(s, pending, ch)
	case "g":
		if keystroke == "g" {
			return gotoFirstLine(s)
		}
		// Second key wasn't g — ignore the prefix silently.
		return s
	}
	return s
}

// --- horizontal motions ---

func stepLeft(s State) State {
	if s.Cursor.Col > 0 {
		s.Cursor.Col--
	}
	s.DesiredCol = -1
	return s
}

func stepRight(s State) State {
	line := s.Buffer.Lines[s.Cursor.Row]
	if s.Cursor.Col < len(line)-1 {
		s.Cursor.Col++
	}
	s.DesiredCol = -1
	return s
}

func stepZero(s State) State {
	s.Cursor.Col = 0
	s.DesiredCol = -1
	return s
}

func stepFirstNonBlank(s State) State {
	line := s.Buffer.Lines[s.Cursor.Row]
	col := 0
	for col < len(line) && charClass(rune(line[col])) == 0 {
		col++
	}
	if col < len(line) {
		s.Cursor.Col = col
	} else {
		s.Cursor.Col = 0
	}
	s.DesiredCol = -1
	return s
}

func stepEndOfLine(s State) State {
	line := s.Buffer.Lines[s.Cursor.Row]
	if len(line) > 0 {
		s.Cursor.Col = len(line) - 1
	} else {
		s.Cursor.Col = 0
	}
	s.DesiredCol = -1
	return s
}

// --- vertical motions ---

// desiredOrCursor returns the desired column if set, otherwise the cursor column.
func desiredOrCursor(s State) int {
	if s.DesiredCol >= 0 {
		return s.DesiredCol
	}
	return s.Cursor.Col
}

// clampToLine clamps a column to the length of the given line.
func clampToLine(line string, col int) int {
	if len(line) == 0 {
		return 0
	}
	if col >= len(line) {
		return len(line) - 1
	}
	return col
}

// verticalMove moves the cursor up or down by delta rows while preserving
// the desired column.
func verticalMove(s State, delta int) State {
	targetRow := s.Cursor.Row + delta
	if targetRow < 0 {
		targetRow = 0
	}
	lastRow := len(s.Buffer.Lines) - 1
	if targetRow > lastRow {
		targetRow = lastRow
	}
	if targetRow < 0 {
		return s
	}

	// The first vertical motion sets desiredCol; subsequent ones preserve it.
	if s.DesiredCol < 0 {
		s.DesiredCol = s.Cursor.Col
	}

	targetLine := s.Buffer.Lines[targetRow]
	s.Cursor.Row = targetRow
	s.Cursor.Col = clampToLine(targetLine, s.DesiredCol)
	return s
}

func stepDown(s State) State {
	return verticalMove(s, 1)
}

func stepUp(s State) State {
	return verticalMove(s, -1)
}

func gotoFirstLine(s State) State {
	if len(s.Buffer.Lines) == 0 {
		return s
	}
	dc := desiredOrCursor(s)
	line := s.Buffer.Lines[0]
	// Set desiredCol before moving so it's recorded.
	if s.DesiredCol < 0 {
		s.DesiredCol = s.Cursor.Col
	}
	s.Cursor.Row = 0
	s.Cursor.Col = clampToLine(line, dc)
	return s
}

func gotoLastLine(s State) State {
	lastRow := len(s.Buffer.Lines) - 1
	if lastRow < 0 {
		return s
	}
	dc := desiredOrCursor(s)
	if s.DesiredCol < 0 {
		s.DesiredCol = s.Cursor.Col
	}
	line := s.Buffer.Lines[lastRow]
	s.Cursor.Row = lastRow
	s.Cursor.Col = clampToLine(line, dc)
	return s
}

// --- word motions ---

// wordForwardStart finds the first character of the next word.
//
// wordFn returns 0 for whitespace, 1+ for non-whitespace character classes.
func wordForwardStart(s State, wordFn func(rune) int) (int, int) {
	row, col := s.Cursor.Row, s.Cursor.Col
	lines := s.Buffer.Lines

	if row >= len(lines) {
		return row, col
	}

	line := lines[row]
	// curClass tracks the character class we are currently "skipping through".
	// 0 = whitespace (look for next non-whitespace), non-zero = in a word/WORD.
	curClass := 0
	if col < len(line) {
		curClass = wordFn(rune(line[col]))
	}

	for {
		col++
		for col >= len(line) {
			row++
			if row >= len(lines) {
				return s.Cursor.Row, s.Cursor.Col
			}
			line = lines[row]
			col = 0
			// End-of-line acts as a word separator: reset to whitespace mode.
			curClass = 0
		}

		cls := wordFn(rune(line[col]))

		if curClass == 0 {
			// Skipping whitespace (or newline) — first non-whitespace is the answer.
			if cls != 0 {
				return row, col
			}
		} else {
			// Skipping through current word — when class changes, switch
			// to whitespace-skip mode or, for adjacent different-class
			// tokens (e.g. "foo.bar"), land on the start of the new token.
			if cls != curClass {
				if cls == 0 {
					curClass = 0
				} else {
					return row, col
				}
			}
		}
	}
}

// wordBackStart finds the first character of the current or previous word.
func wordBackStart(s State, wordFn func(rune) int) (int, int) {
	row, col := s.Cursor.Row, s.Cursor.Col
	lines := s.Buffer.Lines

	if row < 0 || (row == 0 && col <= 0) {
		return 0, 0
	}

	// Helper to get a rune from the buffer, treating out-of-bounds as space.
	charAt := func(r, c int) rune {
		if r < 0 || r >= len(lines) {
			return ' '
		}
		l := lines[r]
		if c < 0 || c >= len(l) {
			return ' '
		}
		return rune(l[c])
	}

	// Step back one character.
	col--
	if col < 0 {
		row--
		if row < 0 {
			return 0, 0
		}
		col = len(lines[row]) - 1
	}

	// Skip whitespace backward.
	cls := wordFn(charAt(row, col))
	for cls == 0 {
		col--
		if col < 0 {
			row--
			if row < 0 {
				return 0, 0
			}
			col = len(lines[row]) - 1
		}
		cls = wordFn(charAt(row, col))
	}

	// Now cls is the class of the word we've landed in. Walk back to its start.
	// Words (or WORDs) do not cross line boundaries: if col == 0 we are
	// already at the start; otherwise step back through same-class chars.
	for col > 0 {
		prevCls := wordFn(charAt(row, col-1))
		if prevCls != cls {
			break
		}
		col--
	}
	return row, col
}

// wordEndForward finds the last character of the current or next word.
func wordEndForward(s State, wordFn func(rune) int) (int, int) {
	row, col := s.Cursor.Row, s.Cursor.Col
	lines := s.Buffer.Lines

	if row >= len(lines) {
		return row, col
	}

	line := lines[row]

	// Helper: is col at the last character of its word on this line?
	isLastInWord := func(r, c int) bool {
		if c >= len(lines[r])-1 {
			return true
		}
		return wordFn(rune(lines[r][c+1])) != wordFn(rune(lines[r][c]))
	}

	// If on whitespace, skip to start of next word, then find its end.
	cls := wordFn(rune(line[col]))
	if cls == 0 {
		nr, nc := wordForwardStart(s, wordFn)
		if nr == row && nc == col {
			return row, col
		}
		return wordEndForward(State{Buffer: s.Buffer, Cursor: Cursor{nr, nc}}, wordFn)
	}

	// If already at the end of the current word, advance to the next word's end.
	if isLastInWord(row, col) {
		// Scan forward for the next non-whitespace (start of next word).
		nc := col + 1
		for {
			if nc >= len(line) {
				row++
				if row >= len(lines) {
					return s.Cursor.Row, s.Cursor.Col
				}
				line = lines[row]
				nc = 0
			}
			if wordFn(rune(line[nc])) != 0 {
				// Found start of next word; recurse.
				return wordEndForward(State{Buffer: s.Buffer, Cursor: Cursor{row, nc}}, wordFn)
			}
			nc++
		}
	}

	// In the middle of a word: advance to its end.
	for {
		nc := col + 1
		if nc >= len(line) {
			return row, col
		}
		if wordFn(rune(line[nc])) != cls {
			return row, col
		}
		col = nc
	}
}

func stepWordForward(s State) State {
	nr, nc := wordForwardStart(s, charClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

func stepWordBack(s State) State {
	nr, nc := wordBackStart(s, charClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

func stepWordEnd(s State) State {
	nr, nc := wordEndForward(s, charClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

func stepWORDForward(s State) State {
	nr, nc := wordForwardStart(s, isWORDClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

func stepWORDBack(s State) State {
	nr, nc := wordBackStart(s, isWORDClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

func stepWORDEnd(s State) State {
	nr, nc := wordEndForward(s, isWORDClass)
	s.Cursor.Row, s.Cursor.Col = nr, nc
	s.DesiredCol = -1
	return s
}

// --- f/t/F/T and ; ---

func applyFT(s State, command string, ch rune) State {
	line := s.Buffer.Lines[s.Cursor.Row]
	col := s.Cursor.Col

	switch command {
	case "f":
		for i := col + 1; i < len(line); i++ {
			if rune(line[i]) == ch {
				s.Cursor.Col = i
				s.DesiredCol = -1
				return s
			}
		}
	case "t":
		for i := col + 1; i < len(line); i++ {
			if rune(line[i]) == ch {
				s.Cursor.Col = i - 1
				s.DesiredCol = -1
				return s
			}
		}
	case "F":
		for i := col - 1; i >= 0; i-- {
			if rune(line[i]) == ch {
				s.Cursor.Col = i
				s.DesiredCol = -1
				return s
			}
		}
	case "T":
		for i := col - 1; i >= 0; i-- {
			if rune(line[i]) == ch {
				s.Cursor.Col = i + 1
				s.DesiredCol = -1
				return s
			}
		}
	}
	return s
}

func repeatLastFT(s State) State {
	if s.LastFT.Command == "" {
		return s
	}
	return applyFT(s, s.LastFT.Command, s.LastFT.Char)
}
