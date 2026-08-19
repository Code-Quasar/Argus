package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultPath returns ~/.argus/store.json, creating the ~/.argus
// directory if it doesn't exist yet.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".argus")
	legacyDir := filepath.Join(home, ".hydra")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if _, legacyErr := os.Stat(legacyDir); legacyErr == nil {
			if err := os.Rename(legacyDir, dir); err != nil {
				return "", fmt.Errorf("migrate %s to %s: %w", legacyDir, dir, err)
			}
		}
	} else if err != nil {
		return "", fmt.Errorf("check %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return filepath.Join(dir, "store.json"), nil
}

func DefaultPIDPath() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "argus.pid"), nil
}

func DefaultLogPath() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "argus.log"), nil
}

// ---------- Data shapes ----------

type KeyRecord struct {
	ID       string    `json:"id"`
	Provider string    `json:"provider"`
	Key      string    `json:"key"` // plaintext — see package doc caveat
	Label    string    `json:"label,omitempty"`
	AddedAt  time.Time `json:"added_at"`
}

// Masked returns the key with everything but the last 4 characters
// hidden, for safe display in CLI output.
func (k KeyRecord) Masked() string {
	if len(k.Key) <= 4 {
		return "****"
	}
	return "..." + k.Key[len(k.Key)-4:]
}

type data struct {
	Keys []KeyRecord `json:"keys"`

	Paused map[string]bool `json:"paused"`

	Priority map[string]int `json:"priority"`

	Stats map[string]map[string]int `json:"stats"`

	CustomModels []CustomModel `json:"custom_models,omitempty"`
}

func emptyData() data {
	return data{
		Paused:   make(map[string]bool),
		Priority: make(map[string]int),
		Stats:    make(map[string]map[string]int),
	}
}
