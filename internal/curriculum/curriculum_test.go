package curriculum_test

import (
	"math/rand"
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/solver"
)

func newTestGenerator(seed int64) *challenge.Generator {
	rng := rand.New(rand.NewSource(seed))
	return challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)
}

func TestNewLesson_Basic(t *testing.T) {
	gen := newTestGenerator(42)

	lesson, err := curriculum.NewLesson(3, gen, challenge.DefaultConfig())
	if err != nil {
		t.Fatalf("NewLesson error: %v", err)
	}

	if len(lesson.Rounds) != 3 {
		t.Fatalf("expected 3 rounds, got %d", len(lesson.Rounds))
	}

	for i, r := range lesson.Rounds {
		if r.Challenge.Par < 0 {
			t.Errorf("round %d: unsolvable (par=%d)", i, r.Challenge.Par)
		}
	}
}

func TestNewLesson_InvalidRoundCount(t *testing.T) {
	gen := newTestGenerator(42)

	_, err := curriculum.NewLesson(0, gen, challenge.DefaultConfig())
	if err == nil {
		t.Fatal("expected error for 0 rounds")
	}

	_, err = curriculum.NewLesson(-1, gen, challenge.DefaultConfig())
	if err == nil {
		t.Fatal("expected error for negative rounds")
	}
}

func TestNewLesson_CyclesTemplates(t *testing.T) {
	gen := newTestGenerator(42)

	lesson, err := curriculum.NewLesson(6, gen, challenge.DefaultConfig())
	if err != nil {
		t.Fatalf("NewLesson error: %v", err)
	}

	// With 3 templates and 6 rounds, each template should appear exactly twice.
	counts := map[string]int{}
	for _, r := range lesson.Rounds {
		counts[r.Template.String()]++
	}
	for _, tmpl := range challenge.Templates() {
		if counts[tmpl.String()] != 2 {
			t.Errorf("template %s appeared %d times, want 2", tmpl, counts[tmpl.String()])
		}
	}
}

func TestComputeSummary(t *testing.T) {
	gen := newTestGenerator(42)

	lesson, err := curriculum.NewLesson(3, gen, challenge.DefaultConfig())
	if err != nil {
		t.Fatalf("NewLesson error: %v", err)
	}

	// Simulate playing rounds by populating results.
	for i := range lesson.Rounds {
		lesson.Rounds[i].Result.Keystrokes = (i + 1) * 5
		lesson.Rounds[i].Result.Par = lesson.Rounds[i].Challenge.Par
		lesson.Rounds[i].Result.Stars = (i + 1) % 4
	}

	summary := lesson.ComputeSummary()
	if summary.TotalKeystrokes != 5+10+15 {
		t.Fatalf("TotalKeystrokes = %d, want 30", summary.TotalKeystrokes)
	}
	if len(summary.Rounds) != 3 {
		t.Fatalf("expected 3 rounds in summary, got %d", len(summary.Rounds))
	}
}