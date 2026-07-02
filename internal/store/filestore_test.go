package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EWMA / Mastery tests
// ---------------------------------------------------------------------------

func TestUpdateMastery_FirstRound(t *testing.T) {
	m := UpdateMastery(Mastery{}, 5, 5, 3, DefaultAlpha)
	if m.Rounds != 1 {
		t.Fatalf("Rounds = %d, want 1", m.Rounds)
	}
	if m.Value <= 0 {
		t.Fatalf("Value = %f, want > 0 for perfect round", m.Value)
	}
	if m.Value > 1.0 {
		t.Fatalf("Value = %f, want <= 1.0", m.Value)
	}
}

func TestUpdateMastery_PerfectRound(t *testing.T) {
	// par=5, keystrokes=5, stars=3 → efficiency=1.0, starScore=1.0 → score=1.0
	m := UpdateMastery(Mastery{}, 5, 5, 3, DefaultAlpha)
	if m.Value != 1.0 {
		t.Fatalf("Value = %f, want 1.0 for perfect round", m.Value)
	}
}

func TestUpdateMastery_PoorRound(t *testing.T) {
	// par=2, keystrokes=10, stars=1 → efficiency=0.2, starScore=0.33 → score≈0.267
	m := UpdateMastery(Mastery{}, 10, 2, 1, DefaultAlpha)
	if m.Value >= 0.5 {
		t.Fatalf("Value = %f, want < 0.5 for poor round", m.Value)
	}
}

func TestUpdateMastery_EWMAConvergence(t *testing.T) {
	// Start with a perfect round, then a poor round.
	m1 := UpdateMastery(Mastery{}, 5, 5, 3, 0.5) // perfect → value=1.0
	m2 := UpdateMastery(m1, 10, 2, 1, 0.5)        // poor

	if m2.Rounds != 2 {
		t.Fatalf("Rounds = %d, want 2", m2.Rounds)
	}
	// m2.Value = 0.5 * 0.267 + 0.5 * 1.0 ≈ 0.633
	if m2.Value < 0.5 || m2.Value > 0.9 {
		t.Fatalf("Value = %f, want around 0.633 after mixed rounds", m2.Value)
	}
}

func TestUpdateMastery_NoParUsesStars(t *testing.T) {
	// par <= 0 falls back to star rating as efficiency.
	m := UpdateMastery(Mastery{}, 5, -1, 3, DefaultAlpha)
	if m.Rounds != 1 {
		t.Fatalf("Rounds = %d, want 1", m.Rounds)
	}
	// With par=-1: efficiency = 5/5 = 1.0 (since par is set to keystrokes)
	// starScore = 1.0, score = 1.0
	if m.Value != 1.0 {
		t.Fatalf("Value = %f, want 1.0", m.Value)
	}
}

func TestUpdateMastery_LastPlayedSet(t *testing.T) {
	m := UpdateMastery(Mastery{}, 5, 5, 3, DefaultAlpha)
	if m.LastPlayed.IsZero() {
		t.Fatal("LastPlayed should be set")
	}
}

// ---------------------------------------------------------------------------
// BestScore tests
// ---------------------------------------------------------------------------

func TestUpdateBestScore_First(t *testing.T) {
	b := UpdateBestScore(BestScore{}, 10, 8, 2)
	if b.Keystrokes != 10 || b.Par != 8 || b.Stars != 2 {
		t.Fatalf("got %+v, want {Keystrokes:10 Par:8 Stars:2}", b)
	}
}

func TestUpdateBestScore_Improves(t *testing.T) {
	current := BestScore{Keystrokes: 10, Par: 8, Stars: 2}
	b := UpdateBestScore(current, 8, 8, 3)
	if b.Stars != 3 || b.Keystrokes != 8 {
		t.Fatalf("got %+v, want better", b)
	}
}

func TestUpdateBestScore_SameStarsFewerKeystrokes(t *testing.T) {
	current := BestScore{Keystrokes: 10, Par: 8, Stars: 2}
	b := UpdateBestScore(current, 9, 8, 2)
	if b.Keystrokes != 9 {
		t.Fatalf("got %+v, want fewer keystrokes at same stars", b)
	}
}

func TestUpdateBestScore_WorseDoesNotReplace(t *testing.T) {
	current := BestScore{Keystrokes: 5, Par: 5, Stars: 3}
	b := UpdateBestScore(current, 10, 5, 2)
	if b.Keystrokes != 5 || b.Stars != 3 {
		t.Fatalf("got %+v, want original best", b)
	}
}

// ---------------------------------------------------------------------------
// FileStore tests
// ---------------------------------------------------------------------------

func TestFileStore_LoadSaveProgress(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "config.toml"),
		filepath.Join(dir, "progress.json"),
	)

	// Load from empty — should get defaults.
	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}

	// Save some progress.
	p.BestScores["hjkl"] = BestScore{Keystrokes: 3, Par: 3, Stars: 3}
	p.Mastery["hjkl"] = Mastery{Value: 0.9, Rounds: 5, LastPlayed: time.Now()}
	if err := fs.SaveProgress(p); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}

	// Load back and verify.
	p2, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress after save: %v", err)
	}
	if p2.BestScores["hjkl"].Keystrokes != 3 {
		t.Fatalf("BestScore.Keystrokes = %d, want 3", p2.BestScores["hjkl"].Keystrokes)
	}
	if p2.Mastery["hjkl"].Rounds != 5 {
		t.Fatalf("Mastery.Rounds = %d, want 5", p2.Mastery["hjkl"].Rounds)
	}
}

func TestFileStore_CorruptProgressReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.json")

	// Write corrupt JSON.
	if err := os.WriteFile(progressPath, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "config.toml"),
		progressPath,
	)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress should not error on corrupt file: %v", err)
	}
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2 (default)", p.Version)
	}
}

func TestFileStore_MissingProgressReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "config.toml"),
		filepath.Join(dir, "nonexistent", "progress.json"),
	)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress should not error on missing file: %v", err)
	}
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}
}

func TestFileStore_EmptyProgressReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.json")

	// Write empty file.
	if err := os.WriteFile(progressPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileStoreWithPaths(filepath.Join(dir, "config.toml"), progressPath)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress should not error on empty file: %v", err)
	}
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}
}

func TestFileStore_ConfigLoadSave(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "config.toml"),
		filepath.Join(dir, "progress.json"),
	)

	// Load default.
	cfg, err := fs.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Save and load back.
	if err := fs.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfg2, err := fs.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	_ = cfg2
}

func TestFileStore_Paths(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "hjkl", "config.toml"),
		filepath.Join(dir, "hjkl", "progress.json"),
	)

	c, p := fs.Paths()
	if c != filepath.Join(dir, "hjkl", "config.toml") {
		t.Fatalf("config path = %q", c)
	}
	if p != filepath.Join(dir, "hjkl", "progress.json") {
		t.Fatalf("progress path = %q", p)
	}
}

// ---------------------------------------------------------------------------
// Integration test: round-trip with actual XDG-like temp dir
// ---------------------------------------------------------------------------

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hjkl", "config.toml")
	progressPath := filepath.Join(dir, "hjkl", "progress.json")

	fs := NewFileStoreWithPaths(configPath, progressPath)

	// Initial state.
	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("initial LoadProgress: %v", err)
	}
	if len(p.BestScores) != 0 || len(p.Mastery) != 0 {
		t.Fatal("expected empty progress")
	}

	// Save.
	p.BestScores["hjkl"] = BestScore{Keystrokes: 3, Par: 3, Stars: 3}
	m := UpdateMastery(Mastery{}, 5, 5, 3, DefaultAlpha)
	p.Mastery["hjkl"] = m
	if err := fs.SaveProgress(p); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(progressPath); os.IsNotExist(err) {
		t.Fatal("progress file was not created")
	}

	// Reload from new FileStore (different instance, same files).
	fs2 := NewFileStoreWithPaths(configPath, progressPath)
	p2, err := fs2.LoadProgress()
	if err != nil {
		t.Fatalf("second LoadProgress: %v", err)
	}
	if p2.BestScores["hjkl"].Stars != 3 {
		t.Errorf("best score stars = %d, want 3", p2.BestScores["hjkl"].Stars)
	}
	if p2.Mastery["hjkl"].Rounds != 1 {
		t.Errorf("mastery rounds = %d, want 1", p2.Mastery["hjkl"].Rounds)
	}
}

func TestFileStore_SaveCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	deepPath := filepath.Join(dir, "a", "b", "c", "progress.json")

	fs := NewFileStoreWithPaths(filepath.Join(dir, "config.toml"), deepPath)
	p := NewProgress()
	p.BestScores["hjkl"] = BestScore{Keystrokes: 5, Par: 5, Stars: 3}

	if err := fs.SaveProgress(p); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}

	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Fatal("deep progress path was not created")
	}
}

func TestFileStore_ConfigMissingReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStoreWithPaths(
		filepath.Join(dir, "nonexistent", "config.toml"),
		filepath.Join(dir, "progress.json"),
	)

	cfg, err := fs.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should not error on missing file: %v", err)
	}
	_ = cfg
}

func TestFileStore_OldVersionResetCleanly(t *testing.T) {
	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.json")

	// Write old-format progress (version 1, template-keyed).
	oldData := `{"version":1,"best_scores":{"horizontal-line":{"keystrokes":3,"par":3,"stars":3}},"mastery":{"horizontal-line":{"value":0.9,"rounds":5}}}`
	if err := os.WriteFile(progressPath, []byte(oldData), 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileStoreWithPaths(filepath.Join(dir, "config.toml"), progressPath)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	// Should be a clean reset: version 2, empty maps.
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}
	if len(p.BestScores) != 0 {
		t.Fatalf("BestScores = %+v, want empty after reset", p.BestScores)
	}
	if len(p.Mastery) != 0 {
		t.Fatalf("Mastery = %+v, want empty after reset", p.Mastery)
	}
}

func TestFileStore_OldVersionResetCleanly(t *testing.T) {
	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.json")

	// Write old-format progress (version 1, template-keyed).
	oldData := `{"version":1,"best_scores":{"horizontal-line":{"keystrokes":3,"par":3,"stars":3}},"mastery":{"horizontal-line":{"value":0.9,"rounds":5}}}`
	if err := os.WriteFile(progressPath, []byte(oldData), 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileStoreWithPaths(filepath.Join(dir, "config.toml"), progressPath)

	p, err := fs.LoadProgress()
	if err != nil {
		t.Fatalf("LoadProgress: %v", err)
	}
	// Should be a clean reset: version 2, empty maps.
	if p.Version != 2 {
		t.Fatalf("Version = %d, want 2", p.Version)
	}
	if len(p.BestScores) != 0 {
		t.Fatalf("BestScores = %+v, want empty after reset", p.BestScores)
	}
	if len(p.Mastery) != 0 {
		t.Fatalf("Mastery = %+v, want empty after reset", p.Mastery)
	}
}

func TestFileStore_CorruptConfigReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(configPath, []byte("{{invalid toml"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := NewFileStoreWithPaths(configPath, filepath.Join(dir, "progress.json"))

	cfg, err := fs.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig should not error on corrupt file: %v", err)
	}
	_ = cfg
}