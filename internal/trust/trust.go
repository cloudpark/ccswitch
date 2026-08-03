// Package trust tracks which repo-level .ccswitch.yaml files the user has
// approved to run automatically, direnv-style. Approval is keyed by a hash of
// the file's content, so editing a trusted file requires re-approval.
package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Store holds the set of trusted repo config files.
type Store struct {
	Trusted map[string]string `yaml:"trusted"` // key: absolute .ccswitch.yaml path, value: sha256 hex of its content
}

const storeFileName = "trusted.yaml"

// getStorePath returns the path to the trust store file.
func getStorePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ccswitch", storeFileName), nil
}

// Load loads the trust store, returning an empty store if it doesn't exist yet.
func Load() (*Store, error) {
	path, err := getStorePath()
	if err != nil {
		return &Store{Trusted: map[string]string{}}, nil
	}

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return &Store{Trusted: map[string]string{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &Store{Trusted: map[string]string{}}, err
	}

	store := &Store{}
	if err := yaml.Unmarshal(data, store); err != nil {
		return &Store{Trusted: map[string]string{}}, err
	}
	if store.Trusted == nil {
		store.Trusted = map[string]string{}
	}

	return store, nil
}

// Save writes the trust store to ~/.ccswitch/trusted.yaml.
func (s *Store) Save() error {
	path, err := getStorePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// HashFile returns the sha256 hex digest of the file at path.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// IsTrusted reports whether path is trusted at the given content hash.
func (s *Store) IsTrusted(path, hash string) bool {
	if s.Trusted == nil {
		return false
	}
	return s.Trusted[path] == hash
}

// Trust records path as trusted at the given content hash.
func (s *Store) Trust(path, hash string) {
	if s.Trusted == nil {
		s.Trusted = map[string]string{}
	}
	s.Trusted[path] = hash
}

// Untrust revokes trust for path.
func (s *Store) Untrust(path string) {
	if s.Trusted == nil {
		return
	}
	delete(s.Trusted, path)
}
