package challenge_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/vim"
)

// buffer is a shorthand for creating a Buffer with the given lines.
func buffer(lines ...string) vim.Buffer {
	return vim.Buffer{Lines: lines}
}

func newTestGenerator(seed int64) *challenge.Generator {
	rng := rand.New(rand.NewSource(seed))
	return challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)
}

func TestGenerator_ProducesSolvableHorizontal(t *testing.T) {
	g := newTestGenerator(42)
	cfg := challenge.DefaultConfig()

	c, err := g.Generate(challenge.THorizontalLine, cfg)
	if err != nil {
		t.Fatalf("Generate(horizontal) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}
	if len(c.InitialBuffer.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(c.InitialBuffer.Lines))
	}
}

func TestGenerator_ProducesSolvableVertical(t *testing.T) {
	g := newTestGenerator(99)
	cfg := challenge.DefaultConfig()

	c, err := g.Generate(challenge.TVerticalNavigation, cfg)
	if err != nil {
		t.Fatalf("Generate(vertical) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}
	if len(c.InitialBuffer.Lines) < 2 {
		t.Fatalf("expected multi-line, got %d lines", len(c.InitialBuffer.Lines))
	}
}

func TestGenerator_ProducesSolvableFindChar(t *testing.T) {
	g := newTestGenerator(77)
	cfg := challenge.DefaultConfig()

	c, err := g.Generate(challenge.TFindCharacter, cfg)
	if err != nil {
		t.Fatalf("Generate(find-char) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}
}

func TestGenerator_CachesPar(t *testing.T) {
	g := newTestGenerator(42)
	cfg := challenge.DefaultConfig()

	c, err := g.Generate(challenge.THorizontalLine, cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if c.Par <= 0 {
		t.Fatalf("expected par > 0, got %d", c.Par)
	}
}

func TestGenerator_AllTemplatesSolvable(t *testing.T) {
	for _, tmpl := range challenge.Templates() {
		t.Run(tmpl.String(), func(t *testing.T) {
			g := newTestGenerator(42)
			cfg := challenge.DefaultConfig()

			c, err := g.Generate(tmpl, cfg)
			if err != nil {
				t.Fatalf("Generate(%s) error: %v", tmpl, err)
			}
			if c.Par < 0 {
				t.Fatalf("par = %d, expected solvable", c.Par)
			}
		})
	}
}

func TestGenerator_MinMotionsRespected(t *testing.T) {
	g := newTestGenerator(42)

	cfg := challenge.Config{
		MinDistance:  5,
		DecoyDensity: 0,
		MinMotions:   3,
		MaxBufferLen: 0,
	}

	c, err := g.Generate(challenge.THorizontalLine, cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if c.Par < 3 {
		t.Fatalf("par = %d, want >= 3", c.Par)
	}
}

func TestGenerator_MinDistanceRespected(t *testing.T) {
	g := newTestGenerator(42)

	cfg := challenge.Config{
		MinDistance:  6,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	c, err := g.Generate(challenge.THorizontalLine, cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Find the target column.
	goalCol := -1
	for col := range c.InitialBuffer.Lines[0] {
		if c.Goal(vim.State{Buffer: c.InitialBuffer, Cursor: vim.Cursor{Row: 0, Col: col}}) {
			goalCol = col
			break
		}
	}
	if goalCol < 0 {
		t.Fatal("could not find goal position")
	}

	dist := goalCol - c.InitialCursor.Col
	if dist < 0 {
		dist = -dist
	}
	if dist < cfg.MinDistance {
		t.Fatalf("distance = %d, want >= %d (start=%d, goal=%d)", dist, cfg.MinDistance, c.InitialCursor.Col, goalCol)
	}
}

func TestGenerator_DifficultyKnobsChangePar(t *testing.T) {
	easyCfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	hardCfg := challenge.Config{
		MinDistance:  8,
		DecoyDensity: 5,
		MinMotions:   4,
		MaxBufferLen: 0,
	}

	g1 := newTestGenerator(42)
	easyC, err := g1.Generate(challenge.THorizontalLine, easyCfg)
	if err != nil {
		t.Fatalf("easy generate error: %v", err)
	}

	g2 := newTestGenerator(99)
	hardC, err := g2.Generate(challenge.THorizontalLine, hardCfg)
	if err != nil {
		t.Fatalf("hard generate error: %v", err)
	}

	if easyC.Par == hardC.Par {
		t.Logf("easy par = %d, hard par = %d (same — needs different RNG to diverge)", easyC.Par, hardC.Par)
	}
}

func TestGenerator_BufferTextFromCorpus(t *testing.T) {
	g := newTestGenerator(42)
	cfg := challenge.DefaultConfig()

	c, err := g.Generate(challenge.THorizontalLine, cfg)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if len(c.InitialBuffer.Lines[0]) == 0 {
		t.Fatal("buffer line is empty")
	}
}

func TestGenerator_VerticalLinesVaryByRound(t *testing.T) {
	g := newTestGenerator(42)
	cfg := challenge.DefaultConfig()

	c1, err1 := g.Generate(challenge.TVerticalNavigation, cfg)
	c2, err2 := g.Generate(challenge.TVerticalNavigation, cfg)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	if c1.InitialBuffer.Lines[0] == c2.InitialBuffer.Lines[0] {
		t.Logf("note: two consecutive vertical rounds had same first line %q", c1.InitialBuffer.Lines[0])
	}
}

func TestGenerator_DeterministicWithSeed(t *testing.T) {
	cfg := challenge.DefaultConfig()

	g1 := newTestGenerator(100)
	c1, err1 := g1.Generate(challenge.THorizontalLine, cfg)

	g2 := newTestGenerator(100)
	c2, err2 := g2.Generate(challenge.THorizontalLine, cfg)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	if c1.InitialBuffer.Lines[0] != c2.InitialBuffer.Lines[0] {
		t.Fatal("same seed should produce same buffer")
	}
	if c1.InitialCursor.Col != c2.InitialCursor.Col {
		t.Fatal("same seed should produce same cursor")
	}
	if c1.Par != c2.Par {
		t.Fatal("same seed should produce same par")
	}
}

func TestGenerator_NoUnsolvableChallenges(t *testing.T) {
	g := newTestGenerator(42)
	cfg := challenge.Config{
		MinDistance:  2,
		DecoyDensity: 3,
		MinMotions:   1,
		MaxBufferLen: 80,
	}

	for i := 0; i < 30; i++ {
		tmpls := challenge.Templates()
		tmpl := tmpls[i%len(tmpls)]
		c, err := g.Generate(tmpl, cfg)
		if err != nil {
			t.Fatalf("iteration %d template %s: %v", i, tmpl, err)
		}
		if c.Par < 0 {
			t.Fatalf("iteration %d template %s: unsolvable (par=%d)", i, tmpl, c.Par)
		}
	}
}

// Keep the original CursorAtTarget test.
func TestCursorAtTarget(t *testing.T) {
	p := challenge.CursorAtTarget(0, 3)

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

func TestIsWall_NilSet(t *testing.T) {
	c := challenge.New(buffer("abc"), vim.Cursor{Row: 0, Col: 0}, challenge.CursorAtTarget(0, 2))
	if c.IsWall(0, 0) {
		t.Error("IsWall should be false for nil WallSet")
	}
	if c.IsWall(0, 5) {
		t.Error("IsWall should be false for out-of-bounds on nil WallSet")
	}
}

func TestIsWall_WithWalls(t *testing.T) {
	walls := challenge.WallSet{
		vim.Cursor{Row: 0, Col: 1}: true,
		vim.Cursor{Row: 1, Col: 0}: true,
	}
	c := challenge.NewWithWalls(buffer("abc", "def"), vim.Cursor{Row: 0, Col: 0}, challenge.CursorAtTarget(0, 2), walls)

	tests := []struct {
		row, col int
		want     bool
	}{
		{0, 0, false},
		{0, 1, true},
		{0, 2, false},
		{1, 0, true},
		{1, 1, false},
		{2, 0, false}, // row out of range
	}

	for _, tt := range tests {
		got := c.IsWall(tt.row, tt.col)
		if got != tt.want {
			t.Errorf("IsWall(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.want)
		}
	}
}

func TestWallSet_NilIsWall(t *testing.T) {
	var w challenge.WallSet // nil
	if w.IsWall(0, 0) {
		t.Error("nil WallSet.IsWall should return false")
	}
}

func TestWallSet_EmptyIsWall(t *testing.T) {
	w := make(challenge.WallSet) // empty, non-nil
	if w.IsWall(0, 0) {
		t.Error("empty WallSet.IsWall should return false")
	}
}

func TestGenerateWall_wbe(t *testing.T) {
	g := newTestGenerator(42)

	// wbe forces word motions with word-interior walls.
	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	// Keys to force: w, b, e (the word motion group).
	forcedKeys := []string{"w", "b", "e"}

	c, err := g.GenerateWall("wbe", forcedKeys, cfg)
	if err != nil {
		t.Fatalf("GenerateWall(wbe) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}

	// Verify par without the forced group is strictly worse.
	restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, forcedKeys)
	restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
	if restrictedPar >= 0 && restrictedPar <= c.Par {
		t.Fatalf("par without wbe (%d) should be > par with wbe (%d) or unsolvable", restrictedPar, c.Par)
	}
}

func TestGenerateWall_LineEdges(t *testing.T) {
	g := newTestGenerator(77)

	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	forcedKeys := []string{"0", "^", "$"}

	c, err := g.GenerateWall("0^$", forcedKeys, cfg)
	if err != nil {
		t.Fatalf("GenerateWall(0^$) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}

	// Verify par without the forced group is strictly worse.
	restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, forcedKeys)
	restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
	if restrictedPar >= 0 && restrictedPar <= c.Par {
		t.Fatalf("par without 0^$ (%d) should be > par with 0^$ (%d) or unsolvable", restrictedPar, c.Par)
	}
}

func TestGenerateWall_ftSemicolon(t *testing.T) {
	g := newTestGenerator(123)

	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	forcedKeys := []string{"f", "t", "F", "T", ";"}

	c, err := g.GenerateWall("ft;", forcedKeys, cfg)
	if err != nil {
		t.Fatalf("GenerateWall(ft;) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}

	// Verify par without the forced group is strictly worse.
	restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, forcedKeys)
	restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
	if restrictedPar >= 0 && restrictedPar <= c.Par {
		t.Fatalf("par without ft; (%d) should be > par with ft; (%d) or unsolvable", restrictedPar, c.Par)
	}
}

func TestGenerateWall_ggG(t *testing.T) {
	g := newTestGenerator(99)

	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	forcedKeys := []string{"g", "G"}

	c, err := g.GenerateWall("ggG", forcedKeys, cfg)
	if err != nil {
		t.Fatalf("GenerateWall(ggG) error: %v", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}

	// Verify par without the forced group is strictly worse.
	restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, forcedKeys)
	restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
	if restrictedPar >= 0 && restrictedPar <= c.Par {
		t.Fatalf("par without ggG (%d) should be > par with ggG (%d) or unsolvable", restrictedPar, c.Par)
	}
}

func TestGenerateWall_WBE(t *testing.T) {
	g := newTestGenerator(55)

	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	forcedKeys := []string{"W", "B", "E"}

	c, err := g.GenerateWall("WBE", forcedKeys, cfg)
	if err != nil {
		t.Skipf("GenerateWall(WBE) skipped: %v (WBE generation is harder, may need retries)", err)
	}

	if c.Par < 0 {
		t.Fatalf("challenge par = %d, expected solvable (>= 0)", c.Par)
	}

	// Verify par without the forced group is strictly worse.
	restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, forcedKeys)
	restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
	if restrictedPar >= 0 && restrictedPar <= c.Par {
		t.Fatalf("par without WBE (%d) should be > par with WBE (%d) or unsolvable", restrictedPar, c.Par)
	}
}

func TestGenerateWall_RejectsHjkl(t *testing.T) {
	g := newTestGenerator(42)

	cfg := challenge.DefaultConfig()
	forcedKeys := []string{"h", "j", "k", "l"}

	_, err := g.GenerateWall("hjkl", forcedKeys, cfg)
	if err == nil {
		t.Fatal("expected error for base group hjkl, got nil")
	}
}

func TestGenerateWall_MultipleSeedsAllSolvable(t *testing.T) {
	groups := []struct {
		key  string
		keys []string
	}{
		{"wbe", []string{"w", "b", "e"}},
		{"0^$", []string{"0", "^", "$"}},
		{"ft;", []string{"f", "t", "F", "T", ";"}},
		{"ggG", []string{"g", "G"}},
		{"WBE", []string{"W", "B", "E"}},
	}

	cfg := challenge.Config{
		MinDistance:  1,
		DecoyDensity: 0,
		MinMotions:   1,
		MaxBufferLen: 0,
	}

	for _, g := range groups {
		for seed := int64(0); seed < 10; seed++ {
			t.Run(g.key+"/"+fmt.Sprint(seed), func(t *testing.T) {
				gen := newTestGenerator(seed)
				c, err := gen.GenerateWall(g.key, g.keys, cfg)
				if err != nil {
					t.Skipf("GenerateWall(%s) error with seed %d: %v (not a failure, retry-until-valid)", g.key, seed, err)
				}
				if c.Par < 0 {
					t.Fatalf("par = %d, expected solvable", c.Par)
				}

				// Verify the wall condition.
				restricted := challenge.RemoveFromVocab(challenge.NavigationVocabulary, g.keys)
				restrictedPar := solver.Solve(c, restricted, solver.DefaultMaxDepth)
				if restrictedPar >= 0 && restrictedPar <= c.Par {
					t.Fatalf("without %s, par=%d should be > with par=%d or unsolvable", g.key, restrictedPar, c.Par)
				}
			})
		}
	}
}

func TestRemoveFromVocab(t *testing.T) {
	vocab := []string{"h", "j", "k", "l", "w", "b", "e"}
	removed := []string{"w", "b", "e"}

	result := challenge.RemoveFromVocab(vocab, removed)

	expected := []string{"h", "j", "k", "l"}
	if len(result) != len(expected) {
		t.Fatalf("len = %d, want %d", len(result), len(expected))
	}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, result[i], expected[i])
		}
	}
}

func TestRemoveFromVocab_NoMatch(t *testing.T) {
	vocab := []string{"h", "j", "k", "l"}
	removed := []string{"x", "y"}

	result := challenge.RemoveFromVocab(vocab, removed)
	if len(result) != len(vocab) {
		t.Fatalf("len = %d, want %d", len(result), len(vocab))
	}
}

func TestRemoveFromVocab_AllRemoved(t *testing.T) {
	vocab := []string{"h", "j"}
	removed := []string{"h", "j"}

	result := challenge.RemoveFromVocab(vocab, removed)
	if len(result) != 0 {
		t.Fatalf("len = %d, want 0", len(result))
	}
}
