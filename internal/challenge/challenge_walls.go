package challenge

import (
	"fmt"
	"strings"

	"github.com/clay/hjkl/internal/vim"
)

// ---------------------------------------------------------------------------
// Wall Challenge generation
// ---------------------------------------------------------------------------

// GenerateWall produces a Wall Challenge that forces the player to use the
// given motion group keys to reach optimal par. It verifies:
//
//	(a) the challenge is solvable with the full vocabulary, and
//	(b) removing the forced keys from the vocabulary makes par strictly
//	    worse (or the challenge unsolvable).
//
// groupKey is used to select the wall layout strategy (e.g. "wbe", "ft;").
// forcedKeys are the motion keystrokes that will be removed during
// verification (typically the keys of the targeted motion group).
func (g *Generator) GenerateWall(groupKey string, forcedKeys []string, cfg Config) (Challenge, error) {
	const maxAttempts = 100

	// Determine which wall strategy to use based on the group key.
	strategy := wallStrategyForGroup(groupKey)
	if strategy < 0 {
		return Challenge{}, fmt.Errorf("no wall strategy for group %q", groupKey)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		c, err := g.tryGenWall(strategy, cfg)
		if err != nil {
			continue
		}

		// Full vocabulary must be able to solve.
		fullPar := g.solver.Solve(c, g.vocabulary, g.maxDepth)
		if fullPar < 0 {
			continue
		}
		if fullPar < cfg.MinMotions {
			continue
		}

		// Without the forced keys, par must be strictly worse or unsolvable.
		restricted := RemoveFromVocab(g.vocabulary, forcedKeys)
		restrictedPar := g.solver.Solve(c, restricted, g.maxDepth)
		if restrictedPar < 0 || restrictedPar > fullPar {
			c.Par = fullPar
			return c, nil
		}
	}

	return Challenge{}, fmt.Errorf("unable to generate wall challenge for group %q after %d attempts", groupKey, maxAttempts)
}

// wallStrategy constants for tryGenWall.
const (
	wallStrategyWordInterior = iota // walls inside a word, e jumps past
	wallStrategyLineEdge            // walls covering most of a line, $/0 bypass
	wallStrategyFindChar            // wall run with f-inducible char past it
	wallStrategyVertical            // walls on entire intermediate lines
	wallStrategyWORD                // punctuation-based WORD walls
)

// wallStrategyForGroup maps a motion group key to a wall layout strategy.
func wallStrategyForGroup(groupKey string) int {
	switch groupKey {
	case "wbe":
		return wallStrategyWordInterior
	case "0^$":
		return wallStrategyLineEdge
	case "ft;":
		return wallStrategyFindChar
	case "ggG":
		return wallStrategyVertical
	case "WBE":
		return wallStrategyWORD
	default:
		return -1
	}
}

// tryGenWall attempts one wall challenge using the given strategy.
func (g *Generator) tryGenWall(strategy int, cfg Config) (Challenge, error) {
	switch strategy {
	case wallStrategyWordInterior:
		return g.genWordInteriorWall(cfg)
	case wallStrategyLineEdge:
		return g.genLineEdgeWall(cfg)
	case wallStrategyFindChar:
		return g.genFindCharWall(cfg)
	case wallStrategyVertical:
		return g.genVerticalWall(cfg)
	case wallStrategyWORD:
		return g.genWORDWall(cfg)
	default:
		return Challenge{}, fmt.Errorf("unknown wall strategy %d", strategy)
	}
}

// RemoveFromVocab returns a new slice with the given keys removed.
func RemoveFromVocab(vocab []string, remove []string) []string {
	removeSet := make(map[string]bool, len(remove))
	for _, k := range remove {
		removeSet[k] = true
	}
	result := make([]string, 0, len(vocab))
	for _, k := range vocab {
		if !removeSet[k] {
			result = append(result, k)
		}
	}
	return result
}

// genWordInteriorWall places walls inside a single word so that character-wise
// motions (h/l) are blocked but word-end motion (e) reaches past them.
// This forces use of the w/b/e group.
func (g *Generator) genWordInteriorWall(cfg Config) (Challenge, error) {
	line := g.allLines[g.rng.Intn(len(g.allLines))]

	// Find a suitable word: at least 6 characters long.
	type wordSpan struct {
		start, end int
	}
	var words []wordSpan
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] < 'a' || line[i] > 'z') {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] >= 'a' && line[i] <= 'z' {
			i++
		}
		if i-start >= 6 {
			words = append(words, wordSpan{start, i})
		}
	}

	if len(words) == 0 {
		return Challenge{}, fmt.Errorf("no long words for word wall")
	}

	word := words[g.rng.Intn(len(words))]
	wordLen := word.end - word.start

	// Place a wall run of about half the word length, in the middle.
	wallLen := wordLen / 2
	if wallLen < 3 {
		wallLen = 3
	}
	if wallLen > wordLen-2 {
		wallLen = wordLen - 2
	}
	wallStart := word.start + (wordLen-wallLen)/2
	wallEnd := wallStart + wallLen

	walls := make(WallSet)
	for col := wallStart; col < wallEnd; col++ {
		walls[vim.Cursor{Row: 0, Col: col}] = true
	}

	startCol := wallStart - 1
	if startCol < word.start {
		startCol = word.start
	}

	targetCol := wallEnd
	if targetCol >= word.end {
		targetCol = word.end - 1
	}

	if startCol == targetCol {
		return Challenge{}, fmt.Errorf("start and target coincide")
	}

	buf := vim.Buffer{Lines: []string{line}}
	return NewWithWalls(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol), walls), nil
}

// genLineEdgeWall places walls covering most of a line so that only line-edge
// motions ($/0/^) efficiently reach the target. Forces 0^$ group.
func (g *Generator) genLineEdgeWall(cfg Config) (Challenge, error) {
	line := g.allLines[g.rng.Intn(len(g.allLines))]
	if len(line) < 8 {
		return Challenge{}, fmt.Errorf("line too short for line-edge wall")
	}

	dir := g.rng.Intn(2)

	var startCol, targetCol int
	walls := make(WallSet)

	if dir == 0 {
		startCol = 0
		targetCol = len(line) - 1
		midStart := len(line) / 5
		midEnd := len(line) - 1 - len(line)/5
		if midEnd-midStart < 3 {
			return Challenge{}, fmt.Errorf("line too short for walls")
		}
		for col := midStart; col < midEnd; col++ {
			walls[vim.Cursor{Row: 0, Col: col}] = true
		}
	} else {
		startCol = len(line) - 1
		targetCol = 0
		midStart := len(line) / 5
		midEnd := len(line) - 1 - len(line)/5
		if midEnd-midStart < 3 {
			return Challenge{}, fmt.Errorf("line too short for walls")
		}
		for col := midStart; col < midEnd; col++ {
			walls[vim.Cursor{Row: 0, Col: col}] = true
		}
	}

	buf := vim.Buffer{Lines: []string{line}}
	return NewWithWalls(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol), walls), nil
}

// genFindCharWall places walls between start and a target character so that
// f/t is the efficient way to reach it.
func (g *Generator) genFindCharWall(cfg Config) (Challenge, error) {
	line := g.allLines[g.rng.Intn(len(g.allLines))]
	if len(line) < 8 {
		return Challenge{}, fmt.Errorf("line too short for find-char wall")
	}

	freq := make(map[byte]int)
	for i := range line {
		if line[i] >= 'a' && line[i] <= 'z' {
			freq[line[i]]++
		}
	}

	var positions []int
	for ch, count := range freq {
		if count >= 2 {
			var pos []int
			for i := range line {
				if line[i] == ch {
					pos = append(pos, i)
				}
			}
			if pos[len(pos)-1]-pos[0] >= 3 {
				positions = pos
				break
			}
		}
	}

	if len(positions) < 2 {
		return Challenge{}, fmt.Errorf("no suitable char for find-char wall")
	}

	targetCol := positions[len(positions)-1]

	walls := make(WallSet)
	if targetCol > 3 {
		for col := 1; col < targetCol; col++ {
			isDecoy := false
			for _, p := range positions[:len(positions)-1] {
				if col == p {
					isDecoy = true
					break
				}
			}
			if !isDecoy {
				walls[vim.Cursor{Row: 0, Col: col}] = true
			}
		}
	}

	startCol := 0
	if startCol == targetCol {
		return Challenge{}, fmt.Errorf("start and target coincide")
	}

	buf := vim.Buffer{Lines: []string{line}}
	return NewWithWalls(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol), walls), nil
}

// genVerticalWall places walls on entire intermediate lines, forcing gg/G
// to jump over them.
func (g *Generator) genVerticalWall(cfg Config) (Challenge, error) {
	var candidates []CorpusEntry
	for _, e := range g.corpus {
		if len(e.Lines) >= 3 {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return Challenge{}, fmt.Errorf("no multi-line entries for vertical wall")
	}

	entry := candidates[g.rng.Intn(len(candidates))]
	nLines := len(entry.Lines)

	dir := g.rng.Intn(2)
	var startRow, targetRow int
	if dir == 0 {
		startRow = 0
		targetRow = nLines - 1
	} else {
		startRow = nLines - 1
		targetRow = 0
	}

	walls := make(WallSet)
	for row := 1; row < nLines-1; row++ {
		for col := 0; col < len(entry.Lines[row]); col++ {
			walls[vim.Cursor{Row: row, Col: col}] = true
		}
	}

	startCol := 0
	if len(entry.Lines[startRow]) > 1 {
		startCol = g.rng.Intn(len(entry.Lines[startRow]))
	}
	targetCol := 0
	if len(entry.Lines[targetRow]) > 0 {
		targetCol = g.rng.Intn(len(entry.Lines[targetRow]))
	}

	if startRow == targetRow {
		return Challenge{}, fmt.Errorf("start and target on same row")
	}

	buf := vim.Buffer{Lines: entry.Lines}
	return NewWithWalls(buf, vim.Cursor{Row: startRow, Col: startCol}, CursorAtTarget(targetRow, targetCol), walls), nil
}

// genWORDWall places walls inside a synthetic WORD with embedded underscores
// so that word motions (w/b/e) stop at underscore boundaries but WORD motions
// (W/B/E) skip the entire span.
func (g *Generator) genWORDWall(cfg Config) (Challenge, error) {
	parts := g.rng.Intn(3) + 3
	var b strings.Builder
	for i := 0; i < parts; i++ {
		if i > 0 {
			b.WriteByte('_')
		}
		partLen := g.rng.Intn(3) + 3
		for j := 0; j < partLen; j++ {
			b.WriteByte(byte('a' + g.rng.Intn(26)))
		}
	}
	wordStr := b.String()
	line := "some prefix " + wordStr + " suffix text"

	wordStart := len("some prefix ")
	wordEnd := wordStart + len(wordStr)

	if wordEnd-wordStart < 8 {
		return Challenge{}, fmt.Errorf("generated WORD too short")
	}

	wordLen := wordEnd - wordStart
	wallLen := wordLen / 2
	if wallLen < 3 {
		wallLen = 3
	}
	if wallLen > wordLen-2 {
		wallLen = wordLen - 2
	}
	wallStartWord := (wordLen - wallLen) / 2
	wallStart := wordStart + wallStartWord
	wallEnd := wallStart + wallLen

	walls := make(WallSet)
	for col := wallStart; col < wallEnd; col++ {
		walls[vim.Cursor{Row: 0, Col: col}] = true
	}

	startCol := wallStart - 1
	if startCol < wordStart {
		startCol = wordStart
	}
	targetCol := wallEnd
	if targetCol >= wordEnd {
		targetCol = wordEnd - 1
	}

	if startCol == targetCol {
		return Challenge{}, fmt.Errorf("start and target coincide")
	}

	buf := vim.Buffer{Lines: []string{line}}
	return NewWithWalls(buf, vim.Cursor{Row: 0, Col: startCol}, CursorAtTarget(0, targetCol), walls), nil
}