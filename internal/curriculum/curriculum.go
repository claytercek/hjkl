// Package curriculum defines a navigation lesson as an ordered sequence of
// generated rounds with a summary of results, plus the motion-group unlock
// system for the learning career.
package curriculum

import (
	"fmt"
	"math/rand"

	"github.com/clay/hjkl/internal/challenge"
	"github.com/clay/hjkl/internal/solver"
	"github.com/clay/hjkl/internal/store"
	"github.com/clay/hjkl/internal/vim"
)

// Result holds the outcome of one round.
type Result struct {
	Keystrokes int
	Par        int // -1 if unknown
	Stars      int
}

// Round holds one challenge within a lesson.
type Round struct {
	Challenge challenge.Challenge
	Template  challenge.TemplateKind
	Result    Result // populated after the round is played
}

// ---------------------------------------------------------------------------
// Motion groups (authored unlock order)
// ---------------------------------------------------------------------------

// MotionGroup is a set of related motions unlocked as a unit.
type MotionGroup struct {
	// ID is a unique identifier (e.g. "basic", "words", "find").
	ID string

	// Name is a display name for the interstitial (e.g. "Word Motions").
	Name string

	// Pitch is shown in the unlock interstitial to explain the motions.
	Pitch string

	// Motions is the set of motion keystrokes in this group.
	Motions []string
}

// AuthoredGroups is the ordered list of all unlockable motion groups.
// Group 0 is always unlocked; each subsequent group unlocks when the
// frontier group's mastery crosses the unlock threshold.
var AuthoredGroups = []MotionGroup{
	{
		ID:   "basic",
		Name: "Basic Motions",
		Pitch: "h moves left, l moves right. " +
			"0 goes to the start of a line, and $ jumps to the end.",
		Motions: []string{"h", "l", "0", "^", "$"},
	},
	{
		ID:   "words",
		Name: "Word Motions",
		Pitch: "w jumps forward a whole word — five l's in one keystroke! " +
			"b jumps back, and e jumps to the end of a word.",
		Motions: []string{"w", "b", "e"},
	},
	{
		ID:   "find",
		Name: "Find Motions",
		Pitch: "f finds a character forward. t finds up to it. " +
			"F and T search backward. ; repeats the last find.",
		Motions: []string{"f", "t", "F", "T", ";"},
	},
	{
		ID:   "vertical",
		Name: "Vertical Motions",
		Pitch: "j and k move down and up lines. " +
			"gg jumps to the first line, G jumps to the last.",
		Motions: []string{"j", "k", "g", "G"},
	},
}

// GroupByID returns the group with the given ID, or nil if not found.
func GroupByID(id string) *MotionGroup {
	for i := range AuthoredGroups {
		if AuthoredGroups[i].ID == id {
			return &AuthoredGroups[i]
		}
	}
	return nil
}

// GroupIndexByID returns the index of the group with the given ID,
// or -1 if not found.
func GroupIndexByID(id string) int {
	for i := range AuthoredGroups {
		if AuthoredGroups[i].ID == id {
			return i
		}
	}
	return -1
}

// VocabularyForUnlocked returns the combined motions for groups [0, count).
func VocabularyForUnlocked(count int) []string {
	if count <= 0 {
		return nil
	}
	if count > len(AuthoredGroups) {
		count = len(AuthoredGroups)
	}
	total := 0
	for i := 0; i < count; i++ {
		total += len(AuthoredGroups[i].Motions)
	}
	vocab := make([]string, 0, total)
	for i := 0; i < count; i++ {
		vocab = append(vocab, AuthoredGroups[i].Motions...)
	}
	return vocab
}

// DefaultUnlockThreshold is the mastery value (0-1) needed to unlock the
// next motion group.
const DefaultUnlockThreshold = 0.7

// ---------------------------------------------------------------------------
// Stream — continuous challenge generation with unlock progression
// ---------------------------------------------------------------------------

// Stream generates challenges on demand for the learning career. It tracks
// which motion groups are unlocked, which is the current frontier, and knows
// how to generate intro rounds for newly unlocked groups.
type Stream struct {
	gen       *challenge.Generator
	cfg       challenge.Config
	rng       *rand.Rand
	progress  store.Progress
	unlocked  int     // number of unlocked groups (1 ≤ unlocked ≤ len(AuthoredGroups))
	threshold float64 // mastery threshold for unlock
}

// NewStream creates a Stream seeded with persisted progress.
// unlockedCount is the number of already-unlocked groups (at least 1).
func NewStream(gen *challenge.Generator, cfg challenge.Config, rng *rand.Rand, progress store.Progress, unlockedCount int) *Stream {
	if unlockedCount < 1 {
		unlockedCount = 1
	}
	if unlockedCount > len(AuthoredGroups) {
		unlockedCount = len(AuthoredGroups)
	}
	if progress.BestScores == nil {
		progress.BestScores = make(map[store.MotionKey]store.BestScore)
	}
	if progress.Mastery == nil {
		progress.Mastery = make(map[store.MotionKey]store.Mastery)
	}
	return &Stream{
		gen:       gen,
		cfg:       cfg,
		rng:       rng,
		progress:  progress,
		unlocked:  unlockedCount,
		threshold: DefaultUnlockThreshold,
	}
}

// FrontierIndex returns the 0-based index of the current frontier group.
func (s *Stream) FrontierIndex() int {
	return s.unlocked - 1
}

// FrontierGroup returns the current frontier motion group.
func (s *Stream) FrontierGroup() MotionGroup {
	return AuthoredGroups[s.unlocked-1]
}

// UnlockedCount returns how many groups are unlocked.
func (s *Stream) UnlockedCount() int {
	return s.unlocked
}

// AllUnlocked returns true when all groups are unlocked.
func (s *Stream) AllUnlocked() bool {
	return s.unlocked >= len(AuthoredGroups)
}

// CurrentVocabulary returns the motion vocabulary for all unlocked groups.
func (s *Stream) CurrentVocabulary() []string {
	return VocabularyForUnlocked(s.unlocked)
}

// NextRound generates the next normal round using the current unlocked
// vocabulary.
func (s *Stream) NextRound() (Round, error) {
	return s.generateRound(s.CurrentVocabulary())
}

// generateRound produces a challenge using the given vocabulary.
// It generates with the full generator, then re-solves with the restricted
// vocabulary to ensure the challenge is solvable and the par is correct.
func (s *Stream) generateRound(vocab []string) (Round, error) {
	const maxAttempts = 50
	templates := challenge.Templates()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		tmpl := templates[s.rng.Intn(len(templates))]
		c, err := s.gen.Generate(tmpl, s.cfg)
		if err != nil {
			continue
		}

		// Re-solve with restricted vocabulary.
		par := solveWithVocab(c, vocab)
		if par < 0 {
			continue // unsolvable with restricted vocabulary
		}
		c.Par = par
		return Round{
			Challenge: c,
			Template:  tmpl,
		}, nil
	}

	return Round{}, fmt.Errorf("unable to generate round with restricted vocabulary after %d attempts", maxAttempts)
}

// FrontierMastery returns the mastery value for the frontier group.
func (s *Stream) FrontierMastery() store.Mastery {
	key := store.MotionKey(AuthoredGroups[s.unlocked-1].ID)
	return s.progress.Mastery[key]
}

// UpdateFrontierMastery updates the frontier group's mastery with a round result.
func (s *Stream) UpdateFrontierMastery(keystrokes, par, stars int) {
	key := store.MotionKey(AuthoredGroups[s.unlocked-1].ID)
	prev := s.progress.Mastery[key]
	s.progress.Mastery[key] = store.UpdateMastery(prev, keystrokes, par, stars, store.DefaultAlpha)
}

// UpdateBestScore updates the best score for the current template key.
func (s *Stream) UpdateBestScore(tmplName string, keystrokes, par, stars int) {
	key := store.MotionKey(tmplName)
	current := s.progress.BestScores[key]
	s.progress.BestScores[key] = store.UpdateBestScore(current, keystrokes, par, stars)
}

// ShouldUnlock returns true if the frontier group's mastery crosses the
// threshold and there is another group to unlock.
func (s *Stream) ShouldUnlock() bool {
	if s.unlocked >= len(AuthoredGroups) {
		return false
	}
	mv := s.FrontierMastery()
	return mv.Rounds > 0 && mv.Value >= s.threshold
}

// UnlockNext advances the unlock state and returns the newly unlocked group.
// Call only when ShouldUnlock returns true.
func (s *Stream) UnlockNext() MotionGroup {
	group := AuthoredGroups[s.unlocked]
	s.unlocked++
	return group
}

// Progress returns the current in-memory progress snapshot.
func (s *Stream) Progress() store.Progress {
	return s.progress
}

// SetProgress replaces the in-memory progress (used when loading from store).
func (s *Stream) SetProgress(p store.Progress) {
	s.progress = p
}

// GenerateIntroRound generates a par-forcing challenge for the given group.
// The challenge is designed so that the optimal solution is strictly better
// when using the new group's motions than without them.
func (s *Stream) GenerateIntroRound(newGroup MotionGroup) (Round, error) {
	// The vocabulary before unlocking the new group.
	prevVocab := VocabularyForUnlocked(s.unlocked - 1)
	// The vocabulary after unlocking.
	newVocab := copyAppend(prevVocab, newGroup.Motions...)

	if len(newGroup.Motions) == 0 {
		// No motions to force — generate a normal round.
		return s.generateRound(newVocab)
	}

	const maxAttempts = 100
	// Use a cfg with larger min distance so word/find/vertical motions matter.
	introCfg := s.cfg
	if introCfg.MinDistance < 10 {
		introCfg.MinDistance = 10
	}
	// Disable the full-vocab MinMotions filter — we'll check par-without-new-group
	// ourselves.
	introCfg.MinMotions = 0

	for attempt := 0; attempt < maxAttempts; attempt++ {
		tmpl := challenge.Templates()[s.rng.Intn(len(challenge.Templates()))]
		c, err := s.gen.Generate(tmpl, introCfg)
		if err != nil {
			continue
		}

		// Check solvability and par with each vocabulary.
		oldPar := solveWithVocab(c, prevVocab)
		newPar := solveWithVocab(c, newVocab)

		if oldPar < 0 || newPar < 0 {
			continue
		}
		if oldPar <= newPar {
			continue // new group doesn't help
		}
		if newPar <= 0 {
			continue
		}

		c.Par = newPar
		return Round{
			Challenge: c,
			Template:  tmpl,
		}, nil
	}

	return Round{}, fmt.Errorf("unable to generate intro round for group %s after %d attempts", newGroup.ID, maxAttempts)
}

// copyAppend returns a new slice with the elements of a followed by b.
func copyAppend(a []string, b ...string) []string {
	result := make([]string, len(a), len(a)+len(b))
	copy(result, a)
	return append(result, b...)
}

// solveWithVocab returns the optimal keystrokes to solve the challenge
// using only the given vocabulary. Returns -1 if unsolvable.
func solveWithVocab(c challenge.Challenge, vocab []string) int {
	st := vim.State{
		Buffer:     c.InitialBuffer,
		Cursor:     c.InitialCursor,
		DesiredCol: -1,
	}
	return solver.OptimalFromState(st, c, vocab, solver.DefaultMaxDepth)
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

// Summary holds per-round and aggregate results after a lesson is complete.
type Summary struct {
	Rounds          []Round
	TotalKeystrokes int
	TotalPar        int // sum of pars (only rounds with par >= 0)
	TotalStars      int
}

// Lesson is an ordered sequence of generated rounds.
type Lesson struct {
	Rounds []Round
}

// NewLesson generates a lesson with the given number of rounds, cycling through
// template types. Each challenge is generated with the provided config.
func NewLesson(numRounds int, gen *challenge.Generator, cfg challenge.Config) (*Lesson, error) {
	if numRounds <= 0 {
		return nil, fmt.Errorf("numRounds must be positive, got %d", numRounds)
	}

	templates := challenge.Templates()
	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates available")
	}

	rounds := make([]Round, numRounds)
	for i := range rounds {
		tmpl := templates[i%len(templates)]
		c, err := gen.Generate(tmpl, cfg)
		if err != nil {
			return nil, fmt.Errorf("round %d (%s): %w", i, tmpl, err)
		}
		rounds[i] = Round{
			Challenge: c,
			Template:  tmpl,
		}
	}

	return &Lesson{Rounds: rounds}, nil
}

// ComputeSummary aggregates results from all completed rounds.
func (l *Lesson) ComputeSummary() Summary {
	s := Summary{
		Rounds: l.Rounds,
	}
	for _, r := range l.Rounds {
		s.TotalKeystrokes += r.Result.Keystrokes
		s.TotalStars += r.Result.Stars
		if r.Result.Par >= 0 {
			s.TotalPar += r.Result.Par
		}
	}
	return s
}