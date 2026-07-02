package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FileStore implements Store using a TOML config file and a JSON progress
// file, both stored at XDG-respecting paths.
//
// Layout:
//
//	~/.config/hjkl/config.toml       (or XDG_CONFIG_HOME)
//	~/.local/share/hjkl/progress.json (or XDG_DATA_HOME)
//
// On macOS and Windows the same layout is used — Go's os.UserConfigDir
// and os.UserHomeDir handle the platform mapping correctly.
type FileStore struct {
	configPath   string
	progressPath string
}

// NewFileStore creates a FileStore rooted at XDG-standard directories.
// The directories are created on first write if they don't exist.
func NewFileStore() (*FileStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("user config dir: %w", err)
	}

	// Use XDG_DATA_HOME or fall back to $HOME/.local/share (per XDG spec).
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user home dir: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	return &FileStore{
		configPath:   filepath.Join(configDir, "hjkl", "config.toml"),
		progressPath: filepath.Join(dataHome, "hjkl", "progress.json"),
	}, nil
}

// NewFileStoreWithPaths creates a FileStore with explicit paths (for testing).
func NewFileStoreWithPaths(configPath, progressPath string) *FileStore {
	return &FileStore{
		configPath:   configPath,
		progressPath: progressPath,
	}
}

// Paths returns the config and progress file paths.
func (f *FileStore) Paths() (string, string) {
	return f.configPath, f.progressPath
}

// LoadProgress reads the progress JSON file. If the file doesn't exist,
// is empty, or is corrupt, it returns an empty Progress with no error.
func (f *FileStore) LoadProgress() (Progress, error) {
	data, err := os.ReadFile(f.progressPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewProgress(), nil
		}
		// Treat read errors as missing data.
		return NewProgress(), nil
	}

	if len(data) == 0 {
		return NewProgress(), nil
	}

	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		// Corrupt file — return defaults.
		return NewProgress(), nil
	}

	if p.BestScores == nil {
		p.BestScores = make(map[MotionKey]BestScore)
	}
	if p.Mastery == nil {
		p.Mastery = make(map[MotionKey]Mastery)
	}

	return p, nil
}

// SaveProgress writes the progress JSON file.
func (f *FileStore) SaveProgress(p Progress) error {
	if err := os.MkdirAll(filepath.Dir(f.progressPath), 0755); err != nil {
		return fmt.Errorf("create progress dir: %w", err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	if err := os.WriteFile(f.progressPath, data, 0644); err != nil {
		return fmt.Errorf("write progress: %w", err)
	}
	return nil
}

// LoadConfig reads the TOML config file. If the file doesn't exist,
// is empty, or is corrupt, it returns DefaultConfig with no error.
func (f *FileStore) LoadConfig() (Config, error) {
	data, err := os.ReadFile(f.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), nil
	}

	if len(data) == 0 {
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), nil
	}

	return cfg, nil
}

// SaveConfig writes the TOML config file.
func (f *FileStore) SaveConfig(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(f.configPath), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	fh, err := os.Create(f.configPath)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer func() { _ = fh.Close() }()

	if err := toml.NewEncoder(fh).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}