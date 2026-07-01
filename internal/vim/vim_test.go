package vim

import (
	"testing"
)

func TestStep_h(t *testing.T) {
	tests := []struct {
		name      string
		initial   State
		wantRow   int
		wantCol   int
	}{
		{
			name:      "h moves left one column",
			initial:   State{Buffer: Buffer{Lines: []string{"hello"}}, Cursor: Cursor{Row: 0, Col: 3}},
			wantRow:   0,
			wantCol:   2,
		},
		{
			name:      "h at column 0 does not move (clamped)",
			initial:   State{Buffer: Buffer{Lines: []string{"hello"}}, Cursor: Cursor{Row: 0, Col: 0}},
			wantRow:   0,
			wantCol:   0,
		},
		{
			name:      "h on empty line stays at column 0",
			initial:   State{Buffer: Buffer{Lines: []string{""}}, Cursor: Cursor{Row: 0, Col: 0}},
			wantRow:   0,
			wantCol:   0,
		},
		{
			name:      "h at column 0 from right edge after multiple hs",
			initial:   State{Buffer: Buffer{Lines: []string{"ab"}}, Cursor: Cursor{Row: 0, Col: 1}},
			wantRow:   0,
			wantCol:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "h")
			if got.Cursor.Row != tt.wantRow || got.Cursor.Col != tt.wantCol {
				t.Errorf("Step(h) = (row=%d, col=%d), want (row=%d, col=%d)",
					got.Cursor.Row, got.Cursor.Col, tt.wantRow, tt.wantCol)
			}
			// Buffer must be unchanged.
			if len(got.Buffer.Lines) != len(tt.initial.Buffer.Lines) {
				t.Errorf("Step(h) modified buffer length")
			}
		})
	}
}

func TestStep_l(t *testing.T) {
	tests := []struct {
		name      string
		initial   State
		wantRow   int
		wantCol   int
	}{
		{
			name:      "l moves right one column",
			initial:   State{Buffer: Buffer{Lines: []string{"hello"}}, Cursor: Cursor{Row: 0, Col: 1}},
			wantRow:   0,
			wantCol:   2,
		},
		{
			name:      "l at end of line does not move (clamped)",
			initial:   State{Buffer: Buffer{Lines: []string{"hello"}}, Cursor: Cursor{Row: 0, Col: 4}},
			wantRow:   0,
			wantCol:   4,
		},
		{
			name:      "l on empty line stays at column 0",
			initial:   State{Buffer: Buffer{Lines: []string{""}}, Cursor: Cursor{Row: 0, Col: 0}},
			wantRow:   0,
			wantCol:   0,
		},
		{
			name:      "l from column 0 on single char line stays",
			initial:   State{Buffer: Buffer{Lines: []string{"x"}}, Cursor: Cursor{Row: 0, Col: 0}},
			wantRow:   0,
			wantCol:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Step(tt.initial, "l")
			if got.Cursor.Row != tt.wantRow || got.Cursor.Col != tt.wantCol {
				t.Errorf("Step(l) = (row=%d, col=%d), want (row=%d, col=%d)",
					got.Cursor.Row, got.Cursor.Col, tt.wantRow, tt.wantCol)
			}
			// Buffer must be unchanged.
			if len(got.Buffer.Lines) != len(tt.initial.Buffer.Lines) {
				t.Errorf("Step(l) modified buffer length")
			}
		})
	}
}

func TestStep_unknownKey(t *testing.T) {
	initial := State{Buffer: Buffer{Lines: []string{"hello"}}, Cursor: Cursor{Row: 0, Col: 2}}
	got := Step(initial, "j")
	if got.Cursor != initial.Cursor {
		t.Errorf("Step(j) should be a no-op, but cursor moved")
	}
}