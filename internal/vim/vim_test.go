package vim

import (
	"testing"
)

// buffer is a shorthand for creating a Buffer with the given lines.
func buffer(lines ...string) Buffer {
	return Buffer{Lines: lines}
}

// state creates a State with the given buffer and cursor position.
// DesiredCol starts unset (-1).
func state(b Buffer, row, col int) State {
	return State{Buffer: b, Cursor: Cursor{Row: row, Col: col}, DesiredCol: -1}
}

func TestStep_h(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "h moves left one column",
			initial: state(buffer("hello"), 0, 3),
			want:    Cursor{0, 2},
		},
		{
			name:    "h at column 0 does not move (clamped)",
			initial: state(buffer("hello"), 0, 0),
			want:    Cursor{0, 0},
		},
		{
			name:    "h on empty line stays at column 0",
			initial: state(buffer(""), 0, 0),
			want:    Cursor{0, 0},
		},
		{
			name:    "h moves left from end of line",
			initial: state(buffer("ab"), 0, 1),
			want:    Cursor{0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "h")
			if got.Cursor != tt.want {
				t.Errorf("Step(h) cursor = %v, want %v", got.Cursor, tt.want)
			}
			if len(got.Buffer.Lines) != len(tt.initial.Buffer.Lines) {
				t.Errorf("Step(h) modified buffer length")
			}
		})
	}
}

func TestStep_l(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "l moves right one column",
			initial: state(buffer("hello"), 0, 1),
			want:    Cursor{0, 2},
		},
		{
			name:    "l at end of line does not move (clamped)",
			initial: state(buffer("hello"), 0, 4),
			want:    Cursor{0, 4},
		},
		{
			name:    "l on empty line stays at column 0",
			initial: state(buffer(""), 0, 0),
			want:    Cursor{0, 0},
		},
		{
			name:    "l from column 0 on single char line stays",
			initial: state(buffer("x"), 0, 0),
			want:    Cursor{0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "l")
			if got.Cursor != tt.want {
				t.Errorf("Step(l) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_j(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		keystrokes []string
		want       Cursor
	}{
		{
			name:       "j moves down one row",
			initial:    state(buffer("abc", "def"), 0, 1),
			keystrokes: []string{"j"},
			want:       Cursor{1, 1},
		},
		{
			name:       "j at bottom row stays",
			initial:    state(buffer("abc", "def"), 1, 1),
			keystrokes: []string{"j"},
			want:       Cursor{1, 1},
		},
		{
			name:       "j clamps to shorter line",
			initial:    state(buffer("abcdef", "xy"), 0, 5),
			keystrokes: []string{"j"},
			want:       Cursor{1, 1},
		},
		{
			name:       "j to empty line goes to column 0",
			initial:    state(buffer("abcdef", ""), 0, 3),
			keystrokes: []string{"j"},
			want:       Cursor{1, 0},
		},
		{
			name:       "j twice preserves desired column",
			initial:    state(buffer("aaaaaa", "bbbbbb", "c"), 0, 3),
			keystrokes: []string{"j", "j"},
			want:       Cursor{2, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initial
			for _, k := range tt.keystrokes {
				got = Step(got, k)
			}
			if got.Cursor != tt.want {
				t.Errorf("step j sequence cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_k(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "k moves up one row",
			initial: state(buffer("abc", "def"), 1, 1),
			want:    Cursor{0, 1},
		},
		{
			name:    "k at top row stays",
			initial: state(buffer("abc", "def"), 0, 1),
			want:    Cursor{0, 1},
		},
		{
			name:    "k clamps to shorter line",
			initial: state(buffer("xy", "abcdef"), 1, 5),
			want:    Cursor{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "k")
			if got.Cursor != tt.want {
				t.Errorf("Step(k) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_0(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "0 goes to column 0",
			initial: state(buffer("hello"), 0, 3),
			want:    Cursor{0, 0},
		},
		{
			name:    "0 at column 0 stays",
			initial: state(buffer("hello"), 0, 0),
			want:    Cursor{0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "0")
			if got.Cursor != tt.want {
				t.Errorf("Step(0) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_hat(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    int // col only, row stays the same
	}{
		{
			name:    "^ moves to first non-blank",
			initial: state(buffer("  hello"), 0, 5),
			want:    2,
		},
		{
			name:    "^ from non-blank stays",
			initial: state(buffer("hello"), 0, 3),
			want:    0,
		},
		{
			name:    "^ on all spaces goes to 0",
			initial: state(buffer("   "), 0, 2),
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "^")
			if got.Cursor.Col != tt.want {
				t.Errorf("Step(^) col = %d, want %d", got.Cursor.Col, tt.want)
			}
		})
	}
}

func TestStep_dollar(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    int // col only
	}{
		{
			name:    "$ goes to end of line",
			initial: state(buffer("hello"), 0, 0),
			want:    4,
		},
		{
			name:    "$ on empty line goes to 0",
			initial: state(buffer(""), 0, 0),
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "$")
			if got.Cursor.Col != tt.want {
				t.Errorf("Step($) col = %d, want %d", got.Cursor.Col, tt.want)
			}
		})
	}
}

func TestStep_w(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "w from start of word goes to next word",
			initial: state(buffer("hello world"), 0, 0),
			want:    Cursor{0, 6},
		},
		{
			name:    "w from middle of word goes to next word",
			initial: state(buffer("hello world"), 0, 2),
			want:    Cursor{0, 6},
		},
		{
			name:    "w from whitespace goes to next word",
			initial: state(buffer("hello  world"), 0, 5),
			want:    Cursor{0, 7},
		},
		{
			name:    "w at last word does not move",
			initial: state(buffer("hello world"), 0, 6),
			want:    Cursor{0, 6},
		},
		{
			name:    "w across lines",
			initial: state(buffer("hello", "world"), 0, 3),
			want:    Cursor{1, 0},
		},
		{
			name:    "w with punctuation boundary",
			initial: state(buffer("foo.bar"), 0, 0),
			want:    Cursor{0, 3}, // '.' is a separate word
		},
		{
			name:    "w from end of word to next",
			initial: state(buffer("abc def"), 0, 2),
			want:    Cursor{0, 4},
		},
		// Word motion and multi-line: w skips end of line to next line
		{
			name:    "w across line boundary with empty line in between",
			initial: state(buffer("hello", "", "world"), 0, 0),
			want:    Cursor{2, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "w")
			if got.Cursor != tt.want {
				t.Errorf("Step(w) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_b(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "b from middle of word goes to word start",
			initial: state(buffer("hello world"), 0, 8),
			want:    Cursor{0, 6},
		},
		{
			name:    "b from start of word goes to previous word start",
			initial: state(buffer("hello world foo"), 0, 12),
			want:    Cursor{0, 6},
		},
		{
			name:    "b from whitespace goes to previous word start",
			initial: state(buffer("hello world"), 0, 5),
			want:    Cursor{0, 0},
		},
		{
			name:    "b at first character stays",
			initial: state(buffer("hello world"), 0, 0),
			want:    Cursor{0, 0},
		},
		{
			name:    "b skips punctuation word boundary",
			initial: state(buffer("foo.bar"), 0, 5),
			want:    Cursor{0, 4}, // start of "." word
		},
		{
			name:    "b across lines backward",
			initial: state(buffer("hello", "world"), 1, 0),
			want:    Cursor{0, 0}, // start of "hello"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "b")
			if got.Cursor != tt.want {
				t.Errorf("Step(b) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_e(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "e from middle of word goes to word end",
			initial: state(buffer("hello world"), 0, 1),
			want:    Cursor{0, 4},
		},
		{
			name:    "e from start of word goes to word end",
			initial: state(buffer("hello world"), 0, 0),
			want:    Cursor{0, 4},
		},
		{
			name:    "e from end of word goes to next word end",
			initial: state(buffer("hello world"), 0, 4),
			want:    Cursor{0, 10},
		},
		{
			name:    "e from whitespace goes to next word end",
			initial: state(buffer("abc def"), 0, 3),
			want:    Cursor{0, 6},
		},
		{
			name:    "e at last word end stays",
			initial: state(buffer("hello world"), 0, 10),
			want:    Cursor{0, 10},
		},
		{
			name:    "e with punctuation boundary",
			initial: state(buffer("foo.bar"), 0, 1),
			want:    Cursor{0, 2}, // end of "foo" ('o')
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "e")
			if got.Cursor != tt.want {
				t.Errorf("Step(e) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_W(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "W from start of WORD goes to next WORD",
			initial: state(buffer("hello world"), 0, 0),
			want:    Cursor{0, 6},
		},
		{
			name:    "W treats punctuation as part of same WORD",
			initial: state(buffer("foo...bar"), 0, 0),
			want:    Cursor{0, 0}, // whole line is one WORD, no next WORD
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "W")
			if got.Cursor != tt.want {
				t.Errorf("Step(W) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_B(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "B from middle of WORD goes to WORD start",
			initial: state(buffer("foo.bar baz"), 0, 8), // 'a' in "baz"
			want:    Cursor{0, 0},                       // start of "foo.bar" WORD
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "B")
			if got.Cursor != tt.want {
				t.Errorf("Step(B) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_E(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "E from middle of WORD goes to WORD end",
			initial: state(buffer("abc.def ghi"), 0, 1),
			want:    Cursor{0, 6}, // end of "abc.def" is 'f'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "E")
			if got.Cursor != tt.want {
				t.Errorf("Step(E) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_f(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		keys       []string // e.g. ["f", "o"]
		want       Cursor
		wantLast   string
		wantLastCh rune
	}{
		{
			name:       "f finds next char",
			initial:    state(buffer("hello world"), 0, 0),
			keys:       []string{"f", "o"},
			want:       Cursor{0, 4},
			wantLast:   "f",
			wantLastCh: 'o',
		},
		{
			name:       "f when char not found stays put",
			initial:    state(buffer("hello"), 0, 0),
			keys:       []string{"f", "z"},
			want:       Cursor{0, 0},
			wantLast:   "f",
			wantLastCh: 'z',
		},
		{
			name:       "f skips current position",
			initial:    state(buffer("aba"), 0, 0),
			keys:       []string{"f", "a"},
			want:       Cursor{0, 2},
			wantLast:   "f",
			wantLastCh: 'a',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			for i, k := range tt.keys {
				s = Step(s, k)
				if i == 0 {
					// After first key, should be pending
					if s.Pending != tt.keys[0] {
						t.Errorf("after first key, pending = %q, want %q", s.Pending, tt.keys[0])
					}
				}
			}
			if s.Cursor != tt.want {
				t.Errorf("f/t sequence cursor = %v, want %v", s.Cursor, tt.want)
			}
			if s.LastFT.Command != tt.wantLast {
				t.Errorf("LastFT.Command = %q, want %q", s.LastFT.Command, tt.wantLast)
			}
			if s.LastFT.Char != tt.wantLastCh {
				t.Errorf("LastFT.Char = %q, want %q", s.LastFT.Char, tt.wantLastCh)
			}
			if s.Pending != "" {
				t.Errorf("after resolve, pending should be empty, got %q", s.Pending)
			}
		})
	}
}

func TestStep_t(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		ch      string
		want    Cursor
	}{
		{
			name:    "t lands before the target char",
			initial: state(buffer("hello world"), 0, 0),
			ch:      "o",
			want:    Cursor{0, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Step(tt.initial, "t")
			s = Step(s, tt.ch)
			if s.Cursor != tt.want {
				t.Errorf("t sequence cursor = %v, want %v", s.Cursor, tt.want)
			}
		})
	}
}

func TestStep_F(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		ch      string
		want    Cursor
	}{
		{
			name:    "F finds previous char",
			initial: state(buffer("hello world"), 0, 6),
			ch:      "o",
			want:    Cursor{0, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Step(tt.initial, "F")
			s = Step(s, tt.ch)
			if s.Cursor != tt.want {
				t.Errorf("F sequence cursor = %v, want %v", s.Cursor, tt.want)
			}
		})
	}
}

func TestStep_T(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		ch      string
		want    Cursor
	}{
		{
			name:    "T lands after the target char",
			initial: state(buffer("hello world"), 0, 6),
			ch:      "o",
			want:    Cursor{0, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Step(tt.initial, "T")
			s = Step(s, tt.ch)
			if s.Cursor != tt.want {
				t.Errorf("T sequence cursor = %v, want %v", s.Cursor, tt.want)
			}
		})
	}
}

func TestStep_semicolon(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name: "; repeats last f command",
			initial: State{
				Buffer:   buffer("a b c"),
				Cursor:   Cursor{0, 0},
				LastFT:   LastFTCommand{Command: "f", Char: 'b'},
				DesiredCol: -1,
			},
			want: Cursor{0, 2},
		},
		{
			name: "; does nothing when no last command",
			initial: State{
				Buffer:     buffer("a b c"),
				Cursor:     Cursor{0, 0},
				DesiredCol: -1,
			},
			want: Cursor{0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, ";")
			if got.Cursor != tt.want {
				t.Errorf("Step(;) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_gg(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		keys       []string
		want       Cursor
	}{
		{
			name:    "gg goes to first line",
			initial: state(buffer("line one", "line two", "line three"), 2, 3),
			keys:    []string{"g", "g"},
			want:    Cursor{0, 3},
		},
		{
			name:    "gg clamps to first line length",
			initial: state(buffer("ab", "bbbbbb"), 1, 5),
			keys:    []string{"g", "g"},
			want:    Cursor{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			for _, k := range tt.keys {
				s = Step(s, k)
			}
			if s.Cursor != tt.want {
				t.Errorf("gg sequence cursor = %v, want %v", s.Cursor, tt.want)
			}
		})
	}
}

func TestStep_G(t *testing.T) {
	tests := []struct {
		name    string
		initial State
		want    Cursor
	}{
		{
			name:    "G goes to last line",
			initial: state(buffer("line one", "line two", "line three"), 0, 0),
			want:    Cursor{2, 0},
		},
		{
			name:    "G clamps to last line length",
			initial: state(buffer("aaaa", "bbbb", "cc"), 0, 3),
			want:    Cursor{2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "G")
			if got.Cursor != tt.want {
				t.Errorf("Step(G) cursor = %v, want %v", got.Cursor, tt.want)
			}
		})
	}
}

func TestStep_visualColumn(t *testing.T) {
	// j/k preserve desired column like vim.
	t.Run("j preserves desired col across lines of varying length", func(t *testing.T) {
		s := state(buffer("abcdef", "xy", "abcdef"), 0, 4)
		s = Step(s, "j") // to row 1 — clamps to col 1, desired col stays 4
		if s.Cursor.Row != 1 || s.Cursor.Col != 1 {
			t.Fatalf("after first j: cursor = %v, want row=1 col=1", s.Cursor)
		}
		s = Step(s, "j") // to row 2 — uses desired col 4
		if s.Cursor.Row != 2 || s.Cursor.Col != 4 {
			t.Fatalf("after second j: cursor = %v, want row=2 col=4", s.Cursor)
		}
	})

	t.Run("horizontal motion resets desired col for subsequent vertical", func(t *testing.T) {
		s := state(buffer("abcdef", "ghijkl", "mnopqr"), 0, 3)
		s = Step(s, "j") // row 1, col 3, desiredCol=3
		s = Step(s, "0") // row 1, col 0, desiredCol=-1
		s = Step(s, "j") // row 2, col 0 (using cursor col, not old desired col)
		if s.Cursor.Row != 2 || s.Cursor.Col != 0 {
			t.Fatalf("after j → 0 → j: cursor = %v, want row=2 col=0", s.Cursor)
		}
	})
}

func TestStep_unknownKey(t *testing.T) {
	initial := state(buffer("hello"), 0, 2)
	got := Step(initial, "x")
	if got.Cursor != initial.Cursor {
		t.Errorf("unknown key should be a no-op, but cursor moved")
	}
	if got.Pending != "" {
		t.Errorf("unknown key should not set pending state")
	}
}

func TestStep_g_nonGG(t *testing.T) {
	initial := state(buffer("hello", "world"), 0, 2)
	s := Step(initial, "g")
	if s.Pending != "g" {
		t.Fatalf("after g, pending = %q, want %q", s.Pending, "g")
	}
	s = Step(s, "x") // gx should be ignored entirely
	if s.Pending != "" {
		t.Errorf("after gx, pending should be empty, got %q", s.Pending)
	}
	if s.Cursor != initial.Cursor {
		t.Errorf("gx moved cursor, want no-op")
	}
}
