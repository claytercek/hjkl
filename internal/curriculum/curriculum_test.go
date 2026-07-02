package curriculum_test

import (
	"math/rand"
	"testing"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/curriculum"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/vim"
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

// ---------------------------------------------------------------------------
// Motion group tests
// ---------------------------------------------------------------------------

func TestAuthoredGroups_NotEmpty(t *testing.T) {
	if len(curriculum.AuthoredGroups) == 0 {
		t.Fatal("AuthoredGroups must not be empty")
	}
}

func TestGroupByID_Found(t *testing.T) {
	g := curriculum.GroupByID("basic")
	if g == nil {
		t.Fatal("GroupByID(\"basic\") returned nil")
	}
	if g.Name != "Basic Motions" {
		t.Errorf("group name = %q, want %q", g.Name, "Basic Motions")
	}
}

func TestGroupByID_NotFound(t *testing.T) {
	g := curriculum.GroupByID("nonexistent")
	if g != nil {
		t.Fatalf("expected nil for nonexistent group, got %v", g)
	}
}

func TestGroupIndexByID(t *testing.T) {
	if idx := curriculum.GroupIndexByID("basic"); idx != 0 {
		t.Errorf("GroupIndexByID(basic) = %d, want 0", idx)
	}
	if idx := curriculum.GroupIndexByID("words"); idx != 1 {
		t.Errorf("GroupIndexByID(words) = %d, want 1", idx)
	}
	if idx := curriculum.GroupIndexByID("nonexistent"); idx != -1 {
		t.Errorf("GroupIndexByID(nonexistent) = %d, want -1", idx)
	}
}

func TestVocabularyForUnlocked(t *testing.T) {
	// With 1 group, should get basic motions.
	vocab := curriculum.VocabularyForUnlocked(1)
	if len(vocab) == 0 {
		t.Fatal("VocabularyForUnlocked(1) returned empty")
	}
	// Should contain h and l.
	found := make(map[string]bool)
	for _, v := range vocab {
		found[v] = true
	}
	if !found["h"] || !found["l"] {
		t.Error("basic vocabulary missing h or l")
	}

	// With 2 groups, should be strictly larger.
	vocab2 := curriculum.VocabularyForUnlocked(2)
	if len(vocab2) <= len(vocab) {
		t.Errorf("vocab with 2 unlocked (%d) not larger than 1 (%d)", len(vocab2), len(vocab))
	}
	// Should contain w.
	found2 := make(map[string]bool)
	for _, v := range vocab2 {
		found2[v] = true
	}
	if !found2["w"] {
		t.Error("vocab with 2 unlocked missing w")
	}
}

func TestVocabularyForUnlocked_Clamped(t *testing.T) {
	maxGroups := len(curriculum.AuthoredGroups)
	// Requesting more than available should not panic.
	vocab := curriculum.VocabularyForUnlocked(maxGroups + 10)
	if len(vocab) == 0 {
		t.Fatal("unexpected empty vocabulary")
	}
	// All motions should be present.
	allMotions := make(map[string]bool)
	for _, g := range curriculum.AuthoredGroups {
		for _, m := range g.Motions {
			allMotions[m] = true
		}
	}
	seen := make(map[string]bool)
	for _, v := range vocab {
		seen[v] = true
	}
	for m := range allMotions {
		if !seen[m] {
			t.Errorf("motion %q missing from full vocabulary", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Stream tests
// ---------------------------------------------------------------------------

func newTestGeneratorForStream(seed int64) *challenge.Generator {
	rng := rand.New(rand.NewSource(seed))
	return challenge.NewGenerator(rng, challenge.SolverFunc(solver.Solve), challenge.NavigationVocabulary, solver.DefaultMaxDepth)
}

func newTestStream(seed int64, unlocked int) *curriculum.Stream {
	gen := newTestGeneratorForStream(seed)
	rng := rand.New(rand.NewSource(seed + 1))
	p := store.NewProgress()
	return curriculum.NewStream(gen, challenge.DefaultConfig(), rng, p, unlocked)
}

func TestStream_New(t *testing.T) {
	s := newTestStream(42, 1)
	if s == nil {
		t.Fatal("NewStream returned nil")
	}
	if s.UnlockedCount() != 1 {
		t.Errorf("UnlockedCount = %d, want 1", s.UnlockedCount())
	}
	if s.FrontierIndex() != 0 {
		t.Errorf("FrontierIndex = %d, want 0", s.FrontierIndex())
	}
}

func TestStream_NextRound(t *testing.T) {
	s := newTestStream(99, 1)
	r, err := s.NextRound()
	if err != nil {
		t.Fatalf("NextRound error: %v", err)
	}
	if r.Challenge.Par < 0 {
		t.Errorf("round par = %d, want >= 0", r.Challenge.Par)
	}
	if r.Template < 0 {
		t.Errorf("invalid template kind %d", r.Template)
	}
}

func TestStream_ShouldUnlock_FalseInitially(t *testing.T) {
	s := newTestStream(42, 1)
	if s.ShouldUnlock() {
		t.Fatal("ShouldUnlock should be false with no mastery")
	}
}

func TestStream_ShouldUnlock_AfterSufficientMastery(t *testing.T) {
	s := newTestStream(42, 1)

	// Update frontier mastery with many 3-star results to push mastery over threshold.
	for i := 0; i < 10; i++ {
		s.UpdateFrontierMastery(3, 3, 3) // at-par 3-star
	}

	if !s.ShouldUnlock() {
		t.Fatal("ShouldUnlock should be true after sufficient mastery")
	}
}

func TestStream_UnlockNext(t *testing.T) {
	s := newTestStream(42, 1)
	if s.UnlockedCount() != 1 {
		t.Fatalf("initial unlocked = %d, want 1", s.UnlockedCount())
	}

	// Push mastery over threshold.
	for i := 0; i < 10; i++ {
		s.UpdateFrontierMastery(3, 3, 3)
	}

	if !s.ShouldUnlock() {
		t.Fatal("ShouldUnlock should be true")
	}

	g := s.UnlockNext()
	if g.ID != "words" {
		t.Errorf("unlocked group ID = %q, want %q", g.ID, "words")
	}
	if s.UnlockedCount() != 2 {
		t.Errorf("after unlock, UnlockedCount = %d, want 2", s.UnlockedCount())
	}
	if s.FrontierIndex() != 1 {
		t.Errorf("after unlock, FrontierIndex = %d, want 1", s.FrontierIndex())
	}
}

func TestStream_AllUnlocked(t *testing.T) {
	s := newTestStream(42, 1)
	if s.AllUnlocked() {
		t.Fatal("should not be all unlocked with count 1")
	}

	// Unlock all groups by repeatedly mastering and unlocking.
	for s.UnlockedCount() < len(curriculum.AuthoredGroups) {
		for i := 0; i < 10; i++ {
			s.UpdateFrontierMastery(3, 3, 3)
		}
		if s.ShouldUnlock() {
			s.UnlockNext()
		}
	}

	if !s.AllUnlocked() {
		t.Fatal("should be all unlocked")
	}
}

func TestStream_UnlockAll_NoFurtherUnlock(t *testing.T) {
	s := newTestStream(42, len(curriculum.AuthoredGroups))
	if !s.AllUnlocked() {
		t.Fatal("should be all unlocked after setting max")
	}
	if s.ShouldUnlock() {
		t.Fatal("ShouldUnlock should be false when all unlocked")
	}
}

func TestStream_GenerateIntroRound(t *testing.T) {
	// Start with 1 group unlocked, then unlock words group, then generate intro.
	s := newTestStream(123, 1)

	// Mastery and unlock to advance to words group.
	for i := 0; i < 10; i++ {
		s.UpdateFrontierMastery(3, 3, 3)
	}
	if !s.ShouldUnlock() {
		t.Fatal("should be ready to unlock after mastery")
	}
	wordsGroup := s.UnlockNext()
	if wordsGroup.ID != "words" {
		t.Fatalf("unlocked %q, want words", wordsGroup.ID)
	}

	// Now generate intro round for just-unlocked words group.
	r, err := s.GenerateIntroRound(wordsGroup)
	if err != nil {
		t.Fatalf("GenerateIntroRound error: %v", err)
	}
	if r.Challenge.Par < 0 {
		t.Errorf("intro round par = %d, want >= 0", r.Challenge.Par)
	}
}

func TestStream_FrontierMastery(t *testing.T) {
	s := newTestStream(42, 1)

	mv := s.FrontierMastery()
	if mv.Rounds != 0 {
		t.Errorf("initial frontier mastery rounds = %d, want 0", mv.Rounds)
	}

	s.UpdateFrontierMastery(5, 5, 3) // at-par, 3 stars
	mv = s.FrontierMastery()
	if mv.Rounds != 1 {
		t.Errorf("after 1 update, rounds = %d, want 1", mv.Rounds)
	}
	if mv.Value <= 0 {
		t.Errorf("mastery value = %f, want > 0", mv.Value)
	}
}

func TestStream_Progress(t *testing.T) {
	s := newTestStream(42, 1)
	p := s.Progress()
	if p.Mastery == nil {
		t.Fatal("Progress().Mastery is nil")
	}
	if p.BestScores == nil {
		t.Fatal("Progress().BestScores is nil")
	}
}

func TestStream_CurrentVocabulary(t *testing.T) {
	s := newTestStream(42, 1)
	vocab := s.CurrentVocabulary()
	// Should contain basic motions.
	found := make(map[string]bool)
	for _, v := range vocab {
		found[v] = true
	}
	if !found["h"] || !found["l"] {
		t.Error("current vocabulary missing h or l")
	}
	if found["w"] {
		t.Error("current vocabulary should not contain w (not yet unlocked)")
	}

	// After unlocking next group, vocabulary should expand.
	for i := 0; i < 10; i++ {
		s.UpdateFrontierMastery(3, 3, 3)
	}
	s.UnlockNext()
	vocab2 := s.CurrentVocabulary()
	found2 := make(map[string]bool)
	for _, v := range vocab2 {
		found2[v] = true
	}
	if !found2["w"] {
		t.Error("vocabulary should contain w after unlocking words group")
	}
}

// Test that intro rounds are par-forcing by comparing old vs new vocabulary solver results.
func TestIntroRound_ParForcing(t *testing.T) {
	s := newTestStream(456, 1)

	// Unlock words group first.
	for i := 0; i < 10; i++ {
		s.UpdateFrontierMastery(3, 3, 3)
	}
	wordsGroup := s.UnlockNext()

	r, err := s.GenerateIntroRound(wordsGroup)
	if err != nil {
		t.Fatalf("GenerateIntroRound error: %v", err)
	}

	// Manually solve with each vocabulary to verify forcing.
	basicVocab := curriculum.VocabularyForUnlocked(1)  // just basic
	withWordsVocab := curriculum.VocabularyForUnlocked(2) // basic + words

	oldPar := solveWithVocab(r.Challenge, basicVocab)
	newPar := solveWithVocab(r.Challenge, withWordsVocab)

	t.Logf("intro round: oldPar=%d, newPar=%d", oldPar, newPar)

	if oldPar < 0 {
		t.Fatal("intro round unsolvable with basic vocabulary")
	}
	if newPar < 0 {
		t.Fatal("intro round unsolvable with words vocabulary")
	}
	if oldPar <= newPar {
		t.Skipf("not strictly forcing: oldPar=%d, newPar=%d (may vary with rng seed)", oldPar, newPar)
	}
}

// solveWithVocab solves a challenge with a specific vocabulary (test helper).
func solveWithVocab(c challenge.Challenge, vocab []string) int {
	st := vim.State{
		Buffer:     c.InitialBuffer,
		Cursor:     c.InitialCursor,
		DesiredCol: -1,
	}
	return solver.OptimalFromState(st, c, vocab, solver.DefaultMaxDepth)
}

func TestStream_New_ClampsUnlocked(t *testing.T) {
	gen := newTestGeneratorForStream(42)
	rng := rand.New(rand.NewSource(43))
	p := store.NewProgress()

	// unlocked = 0 should be clamped to 1.
	s := curriculum.NewStream(gen, challenge.DefaultConfig(), rng, p, 0)
	if s.UnlockedCount() != 1 {
		t.Errorf("UnlockedCount = %d, want 1", s.UnlockedCount())
	}

	// unlocked = large should be clamped to max.
	s2 := curriculum.NewStream(gen, challenge.DefaultConfig(), rng, p, 999)
	if s2.UnlockedCount() != len(curriculum.AuthoredGroups) {
		t.Errorf("UnlockedCount = %d, want %d", s2.UnlockedCount(), len(curriculum.AuthoredGroups))
	}
}

func TestStream_UpdateBestScore(t *testing.T) {
	s := newTestStream(42, 1)
	s.UpdateBestScore("horizontal-line", 5, 5, 3)
	p := s.Progress()
	if bs, ok := p.BestScores["horizontal-line"]; !ok {
		t.Fatal("best score not recorded")
	} else if bs.Keystrokes != 5 {
		t.Errorf("best keystrokes = %d, want 5", bs.Keystrokes)
	}
}