// Package store persists progress and configuration data for hjkl.
//
// The Store interface abstracts local persistence so the backing format
// (JSON, SQLite, etc.) is swappable without changing callers.
package store

import "time"

// MotionKey identifies a motion type (e.g. "horizontal-line",
// "vertical-navigation", "find-character").
type MotionKey string

// BestScore holds the best result ever achieved for a motion type.
type BestScore struct {
	Keystrokes int `json:"keystrokes"`
	Par        int `json:"par"`
	Stars      int `json:"stars"`
}

// Mastery tracks proficiency for a motion type using an exponentially-
// weighted moving average of efficiency (keystrokes vs par) and speed.
//
//	Value ∈ [0, 1] where 1 = perfect.
//	Rounds is the number of completed rounds that contributed.
type Mastery struct {
	Value      float64   `json:"value"`
	Rounds     int       `json:"rounds"`
	LastPlayed time.Time `json:"last_played"`
}

// Progress is the complete set of persisted progress data.
type Progress struct {
	Version    int                  `json:"version"`    // schema version for forward compat
	BestScores map[MotionKey]BestScore `json:"best_scores,omitempty"`
	Mastery    map[MotionKey]Mastery   `json:"mastery,omitempty"`
}

// Config holds user preferences persisted in TOML.
type Config struct {
	// RoundsPerLesson is the number of rounds in a lesson.
	RoundsPerLesson int `toml:"rounds_per_lesson"`

	// Theme is an optional UI theme identifier.
	Theme string `toml:"theme,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		RoundsPerLesson: 5,
	}
}

// Store is the persistence interface for hjkl progress and config.
//
// Implementations must be safe for concurrent reads but need not be
// safe for concurrent writes (callers serialize access).
type Store interface {
	// LoadProgress reads persisted progress. Returns a zero-value Progress
	// if no data exists yet or the file is corrupt.
	LoadProgress() (Progress, error)

	// SaveProgress persists progress.
	SaveProgress(p Progress) error

	// LoadConfig reads persisted config. Returns DefaultConfig if no
	// data exists yet or the file is corrupt.
	LoadConfig() (Config, error)

	// SaveConfig persists config.
	SaveConfig(c Config) error

	// Paths returns the file paths used by this store (for diagnostics).
	Paths() (configPath, progressPath string)
}

// NewProgress returns an initialised Progress with empty maps.
func NewProgress() Progress {
	return Progress{
		Version:    1,
		BestScores: make(map[MotionKey]BestScore),
		Mastery:    make(map[MotionKey]Mastery),
	}
}