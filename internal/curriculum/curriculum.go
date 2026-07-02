// Package curriculum defines a navigation lesson as an ordered sequence of
// generated rounds with a summary of results.
package curriculum

import (
	"fmt"

	"github.com/clay/hjkl/internal/challenge"
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

// Summary holds per-round and aggregate results after a lesson is complete.
type Summary struct {
	Rounds      []Round
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