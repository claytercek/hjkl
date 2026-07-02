// Package challenge defines the atomic unit of practice (ADR 0002).
//
// A Challenge carries a starting buffer and cursor together with a goal
// predicate that signals when the player has succeeded. It has no
// dependency on Bubble Tea or any TUI library.
package challenge

import (
	"fmt"
	"math/rand"

	"github.com/clay/hjkl/internal/vim"
)

// GoalPredicate returns true when the given state satisfies the challenge.
type GoalPredicate func(vim.State) bool

// WallSet marks buffer cells the cursor cannot enter. The zero value (nil)
// represents no walls. A nil WallSet behaves as an empty set.
type WallSet map[vim.Cursor]bool

// IsWall reports whether the given position is a wall cell.
func (w WallSet) IsWall(row, col int) bool {
	return w[vim.Cursor{Row: row, Col: col}]
}

// Challenge is the atomic unit of practice.
type Challenge struct {
	// InitialBuffer is the buffer the player starts with.
	InitialBuffer vim.Buffer

	// InitialCursor is the starting cursor position.
	InitialCursor vim.Cursor

	// Goal returns true when the player has solved the challenge.
	Goal GoalPredicate

	// Par is the optimal (minimum) keystrokes to solve, computed by the
	// solver. -1 means no solution was found. 0 means unset.
	Par int

	// Walls marks cells the cursor cannot land on. When a motion would land
	// on a wall, the cursor stays at its previous position but the keystroke
	// still counts. Motions that jump over a wall (word, find, line) work
	// normally as long as their landing cell is clear.
	Walls WallSet
}

// IsWall reports whether the given buffer cell is blocked by a wall.
// Returns false when Walls is nil (the default).
func (c Challenge) IsWall(row, col int) bool {
	return c.Walls.IsWall(row, col)
}

// New returns a Challenge with the given starting state and goal predicate.
// Par is left at its zero value (0); set it explicitly after solving.
func New(buf vim.Buffer, cursor vim.Cursor, goal GoalPredicate) Challenge {
	return Challenge{
		InitialBuffer: buf,
		InitialCursor: cursor,
		Goal:          goal,
	}
}

// NewWithPar is like New but also sets the par.
func NewWithPar(buf vim.Buffer, cursor vim.Cursor, goal GoalPredicate, par int) Challenge {
	c := New(buf, cursor, goal)
	c.Par = par
	return c
}

// NewWithWalls is like New but also sets wall cells.
func NewWithWalls(buf vim.Buffer, cursor vim.Cursor, goal GoalPredicate, walls WallSet) Challenge {
	c := New(buf, cursor, goal)
	c.Walls = walls
	return c
}

// CursorAtTarget returns a GoalPredicate that is satisfied when the cursor
// is at the given position.
func CursorAtTarget(row, col int) GoalPredicate {
	return func(s vim.State) bool {
		return s.Cursor.Row == row && s.Cursor.Col == col
	}
}

// ---------------------------------------------------------------------------
// Generator — parameterized challenge generation from templates
// ---------------------------------------------------------------------------

// TemplateKind identifies the pattern used to generate a challenge.
type TemplateKind int

const (
	// THorizontalLine generates a single-line challenge: navigate horizontally
	// using l/h/w/b/e/0/^/$/f/t to reach a target position.
	THorizontalLine TemplateKind = iota

	// TVerticalNavigation generates a multi-line challenge: move between lines
	// using j/k/gg/G combined with horizontal motion.
	TVerticalNavigation

	// TFindCharacter generates a challenge where the player must use f/t/F/T
	// to find a specific character occurrence. Decoy density controls how many
	// other occurrences of the same character exist before the target.
	TFindCharacter
)

// String returns a human-readable name for the template.
func (k TemplateKind) String() string {
	switch k {
	case THorizontalLine:
		return "horizontal-line"
	case TVerticalNavigation:
		return "vertical-navigation"
	case TFindCharacter:
		return "find-character"
	default:
		return fmt.Sprintf("template(%d)", int(k))
	}
}

// Config controls challenge generation.
type Config struct {
	// MinDistance is the minimum Manhattan distance from cursor to target.
	MinDistance int

	// DecoyDensity controls how many similar characters or words appear
	// between the cursor and the target, making f/t motions harder.
	// 0 = unique target, 10 = many decoys.
	DecoyDensity int

	// MinMotions is the minimum solver-computed par. The generator retries
	// until par >= MinMotions. Caller must call SolvePar after generation.
	MinMotions int

	// MaxBufferLen is the approximate maximum total characters in the buffer.
	// 0 means no limit.
	MaxBufferLen int
}

// DefaultConfig returns a moderate-difficulty config.
func DefaultConfig() Config {
	return Config{
		MinDistance:  3,
		DecoyDensity: 2,
		MinMotions:   2,
		MaxBufferLen: 200,
	}
}

// NavigationVocabulary is the full motion vocabulary used for par computation.
var NavigationVocabulary = []string{
	"h", "j", "k", "l",
	"0", "^", "$",
	"w", "b", "e",
	"W", "B", "E",
	"f", "t", "F", "T", ";",
	"g", "G",
	// ASCII letters for f/t/F/T targets.
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
	"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	// Include period for punctuation-heavy buffers.
	".",
}

// Solver is the interface the generator uses to verify solvability and
// compute par. This lets the generator avoid importing the solver package
// directly, breaking the potential import cycle (solver imports challenge).
type Solver interface {
	Solve(c Challenge, vocabulary []string, maxDepth int) int
}

// SolverFunc adapts a function to the Solver interface.
type SolverFunc func(Challenge, []string, int) int

func (f SolverFunc) Solve(c Challenge, vocabulary []string, maxDepth int) int {
	return f(c, vocabulary, maxDepth)
}

// Generator produces fresh challenges from templates.
type Generator struct {
	rng        *rand.Rand
	corpus     []CorpusEntry
	allLines   []string
	vocabulary []string
	solver     Solver
	maxDepth   int
}

// NewGenerator creates a Generator that draws buffer text from the curated
// corpus and uses the given solver, vocabulary, and max depth for par
// computation. If vocabulary is nil, NavigationVocabulary is used.
func NewGenerator(rng *rand.Rand, solver Solver, vocabulary []string, maxDepth int) *Generator {
	if vocabulary == nil {
		vocabulary = NavigationVocabulary
	}
	return &Generator{
		rng:        rng,
		corpus:     curatedCorpus,
		allLines:   curatedLines,
		vocabulary: vocabulary,
		solver:     solver,
		maxDepth:   maxDepth,
	}
}

// Generate produces a challenge using the given template, configured by cfg.
// It guarantees the challenge is solvable and caches the par.
func (g *Generator) Generate(t TemplateKind, cfg Config) (Challenge, error) {
	const maxAttempts = 100

	for attempt := 0; attempt < maxAttempts; attempt++ {
		c, err := g.tryGenerate(t, cfg)
		if err != nil {
			continue
		}

		par := g.solver.Solve(c, g.vocabulary, g.maxDepth)
		if par < 0 {
			continue
		}
		if par < cfg.MinMotions {
			continue
		}
		c.Par = par
		return c, nil
	}

	return Challenge{}, fmt.Errorf("unable to generate %s challenge meeting config %+v after %d attempts", t, cfg, maxAttempts)
}

// tryGenerate attempts one challenge without solving it.
func (g *Generator) tryGenerate(t TemplateKind, cfg Config) (Challenge, error) {
	switch t {
	case THorizontalLine:
		return g.genHorizontal(cfg)
	case TVerticalNavigation:
		return g.genVertical(cfg)
	case TFindCharacter:
		return g.genFindChar(cfg)
	default:
		return Challenge{}, fmt.Errorf("unknown template %d", t)
	}
}

// genHorizontal generates a single-line horizontal navigation challenge.
func (g *Generator) genHorizontal(cfg Config) (Challenge, error) {
	line := g.allLines[g.rng.Intn(len(g.allLines))]
	if cfg.MaxBufferLen > 0 && len(line) > cfg.MaxBufferLen {
		if cfg.MaxBufferLen < 5 {
			return Challenge{}, fmt.Errorf("max buffer too small")
		}
		line = line[:cfg.MaxBufferLen]
		// Back up to a word boundary.
		for len(line) > 0 && line[len(line)-1] != ' ' {
			line = line[:len(line)-1]
		}
		if len(line) < 5 {
			return Challenge{}, fmt.Errorf("buffer too short after truncation")
		}
	}

	if len(line) < 3 {
		return Challenge{}, fmt.Errorf("line too short")
	}

	// Ensure line is long enough to accommodate MinDistance.
	if len(line) <= cfg.MinDistance {
		return Challenge{}, fmt.Errorf("line length %d too short for min distance %d", len(line), cfg.MinDistance)
	}

	// Pick start and target positions along the line.
	startCol := g.rng.Intn(len(line))
	targetCol := g.rng.Intn(len(line))
	for i := 0; i < 100; i++ {
		if absDiff(startCol, targetCol) >= cfg.MinDistance && targetCol != startCol {
			break
		}
		targetCol = g.rng.Intn(len(line))
	}
	if absDiff(startCol, targetCol) < cfg.MinDistance || targetCol == startCol {
		// Force a valid pair.
		if startCol+cfg.MinDistance < len(line) {
			targetCol = startCol + cfg.MinDistance
		} else if startCol-cfg.MinDistance >= 0 {
			targetCol = startCol - cfg.MinDistance
		} else {
			return Challenge{}, fmt.Errorf("cannot place target at distance %d from start %d on line of length %d", cfg.MinDistance, startCol, len(line))
		}
	}

	buf := vim.Buffer{Lines: []string{line}}
	c := New(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol))
	return c, nil
}

// genVertical generates a multi-line navigation challenge.
func (g *Generator) genVertical(cfg Config) (Challenge, error) {
	// Collect all multi-line entries from the corpus that fit within MaxBufferLen.
	var candidates []CorpusEntry
	for _, e := range g.corpus {
		if len(e.Lines) < 3 {
			continue
		}
		if cfg.MaxBufferLen > 0 {
			total := 0
			for _, l := range e.Lines {
				total += len(l)
			}
			if total > cfg.MaxBufferLen {
				continue
			}
		}
		candidates = append(candidates, e)
	}

	if len(candidates) == 0 {
		// Fall back: build one from individual lines.
		n := 3 + g.rng.Intn(3)
		if n < 2 {
			n = 2
		}
		lines := make([]string, n)
		for i := range lines {
			lines[i] = g.allLines[g.rng.Intn(len(g.allLines))]
		}
		// If max buffer is set, truncate the set of lines to fit.
		if cfg.MaxBufferLen > 0 {
			total := 0
			keep := 0
			for i, l := range lines {
				if total+len(l) > cfg.MaxBufferLen {
					break
				}
				total += len(l)
				keep = i + 1
			}
			if keep < 2 {
				return Challenge{}, fmt.Errorf("lines too long for max buffer")
			}
			lines = lines[:keep]
		}
		candidates = []CorpusEntry{{Lines: lines}}
	}

	entry := candidates[g.rng.Intn(len(candidates))]

	nLines := len(entry.Lines)
	if nLines < 2 {
		return Challenge{}, fmt.Errorf("need at least 2 lines for vertical")
	}

	startRow := g.rng.Intn(nLines)
	targetRow := g.rng.Intn(nLines)
	for targetRow == startRow {
		targetRow = g.rng.Intn(nLines)
	}

	startLine := entry.Lines[startRow]
	targetLine := entry.Lines[targetRow]

	startCol := 0
	if len(startLine) > 0 {
		startCol = g.rng.Intn(len(startLine))
	}
	targetCol := 0
	if len(targetLine) > 0 {
		targetCol = g.rng.Intn(len(targetLine))
	}

	dist := absDiff(startRow, targetRow) + absDiff(startCol, targetCol)
	if dist < cfg.MinDistance {
		return Challenge{}, fmt.Errorf("distance too short")
	}

	buf := vim.Buffer{Lines: entry.Lines}
	c := New(buf, vim.Cursor{Row: startRow, Col: startCol}, CursorAtTarget(targetRow, targetCol))
	return c, nil
}

// genFindChar generates a challenge that rewards using f/t motions.
func (g *Generator) genFindChar(cfg Config) (Challenge, error) {
	line := g.allLines[g.rng.Intn(len(g.allLines))]
	if cfg.MaxBufferLen > 0 && len(line) > cfg.MaxBufferLen {
		line = line[:cfg.MaxBufferLen]
		for len(line) > 0 && line[len(line)-1] != ' ' {
			line = line[:len(line)-1]
		}
		if len(line) < 5 {
			return Challenge{}, fmt.Errorf("buffer too short")
		}
	}

	if len(line) < 4 {
		return Challenge{}, fmt.Errorf("line too short for f challenge")
	}

	// Count character frequencies.
	freq := make(map[byte]int)
	for i := range line {
		if line[i] >= 'a' && line[i] <= 'z' {
			freq[line[i]]++
		}
	}

	// Find characters that appear frequently enough for decoys.
	var candidates []byte
	if cfg.DecoyDensity <= 2 {
		for ch, count := range freq {
			if count >= 1 && count <= 3 {
				candidates = append(candidates, ch)
			}
		}
	} else {
		for ch, count := range freq {
			if count >= 2 {
				candidates = append(candidates, ch)
			}
		}
	}

	if len(candidates) == 0 {
		for ch := range freq {
			candidates = append(candidates, ch)
		}
	}
	if len(candidates) == 0 {
		return Challenge{}, fmt.Errorf("no letter candidates in line")
	}

	targetChar := candidates[g.rng.Intn(len(candidates))]

	// Find all occurrences of targetChar.
	var positions []int
	for i := range line {
		if line[i] == targetChar {
			positions = append(positions, i)
		}
	}
	if len(positions) == 0 {
		return Challenge{}, fmt.Errorf("no occurrences of target char")
	}

	// Pick a target occurrence. With higher decoy density, pick one
	// that has many preceding occurrences (making f harder).
	var targetIdx int
	if cfg.DecoyDensity <= 3 || len(positions) <= 1 {
		targetIdx = g.rng.Intn(len(positions))
	} else {
		minIdx := len(positions) - 1 - cfg.DecoyDensity
		if minIdx < 0 {
			minIdx = 0
		}
		if minIdx >= len(positions) {
			minIdx = len(positions) - 1
		}
		n := len(positions) - minIdx
		if n <= 0 {
			targetIdx = minIdx
		} else {
			targetIdx = minIdx + g.rng.Intn(n)
		}
		if targetIdx >= len(positions) {
			targetIdx = len(positions) - 1
		}
	}

	targetCol := positions[targetIdx]

	// Pick a start position before the target occurrence.
	var startCol int
	if targetCol > 0 {
		startCol = g.rng.Intn(targetCol)
	}
	if targetCol-startCol < cfg.MinDistance {
		if targetCol > cfg.MinDistance {
			startCol = targetCol - cfg.MinDistance
		} else {
			startCol = 0
		}
	}

	buf := vim.Buffer{Lines: []string{line}}
	c := New(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol))
	return c, nil
}

// Templates returns the list of template kinds the generator supports.
func Templates() []TemplateKind {
	return []TemplateKind{THorizontalLine, TVerticalNavigation, TFindCharacter}
}

// absDiff returns the absolute difference between two integers.
func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}