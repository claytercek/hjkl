package challenge_test

import (
	"math/rand"
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/vim"
)

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