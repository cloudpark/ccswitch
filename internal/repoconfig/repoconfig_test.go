package repoconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepoConfig_NoFile(t *testing.T) {
	tempDir := t.TempDir()

	cfg, err := LoadRepoConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig() with no file failed: %v", err)
	}
	if len(cfg.PostCreate.Commands) != 0 {
		t.Errorf("expected no commands, got %v", cfg.PostCreate.Commands)
	}
}

func TestLoadRepoConfig_ValidFile(t *testing.T) {
	tempDir := t.TempDir()

	content := "post_create:\n  commands:\n    - echo one\n    - echo two\n"
	if err := os.WriteFile(GetRepoConfigPath(tempDir), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadRepoConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig() failed: %v", err)
	}

	expected := []string{"echo one", "echo two"}
	if len(cfg.PostCreate.Commands) != len(expected) {
		t.Fatalf("expected %d commands, got %d", len(expected), len(cfg.PostCreate.Commands))
	}
	for i, c := range expected {
		if cfg.PostCreate.Commands[i] != c {
			t.Errorf("Commands[%d] = %q, expected %q", i, cfg.PostCreate.Commands[i], c)
		}
	}
}

func TestLoadRepoConfig_EmptyCommandsList(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"no post_create section", ""},
		{"empty post_create section", "post_create: {}\n"},
		{"explicit empty commands list", "post_create:\n  commands: []\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if err := os.WriteFile(GetRepoConfigPath(tempDir), []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := LoadRepoConfig(tempDir)
			if err != nil {
				t.Fatalf("LoadRepoConfig() failed: %v", err)
			}
			if len(cfg.PostCreate.Commands) != 0 {
				t.Errorf("expected no commands, got %v", cfg.PostCreate.Commands)
			}
		})
	}
}

func TestLoadRepoConfig_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()

	if err := os.WriteFile(GetRepoConfigPath(tempDir), []byte("not: valid: yaml: :::"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadRepoConfig(tempDir)
	if err == nil {
		t.Fatal("expected an error for invalid YAML, got nil")
	}
	if cfg == nil {
		t.Fatal("expected a non-nil default config even on error")
	}
	if len(cfg.PostCreate.Commands) != 0 {
		t.Errorf("expected no commands in default config, got %v", cfg.PostCreate.Commands)
	}
}

func TestGetRepoConfigPath(t *testing.T) {
	repoPath := "/some/repo"
	expected := filepath.Join(repoPath, ".ccswitch.yaml")
	if got := GetRepoConfigPath(repoPath); got != expected {
		t.Errorf("GetRepoConfigPath() = %q, expected %q", got, expected)
	}
}

func TestScaffold_CreatesFile(t *testing.T) {
	tempDir := t.TempDir()

	path, err := Scaffold(tempDir)
	if err != nil {
		t.Fatalf("Scaffold() failed: %v", err)
	}
	if path != GetRepoConfigPath(tempDir) {
		t.Errorf("Scaffold() path = %q, expected %q", path, GetRepoConfigPath(tempDir))
	}

	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected scaffolded file to exist: %v", statErr)
	}

	// Scaffolded content is commented-out YAML, so it should parse to an empty config.
	cfg, err := LoadRepoConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig() on scaffolded file failed: %v", err)
	}
	if len(cfg.PostCreate.Commands) != 0 {
		t.Errorf("expected no commands from scaffolded template, got %v", cfg.PostCreate.Commands)
	}
}

func TestScaffold_DoesNotOverwrite(t *testing.T) {
	tempDir := t.TempDir()

	if _, err := Scaffold(tempDir); err != nil {
		t.Fatalf("first Scaffold() failed: %v", err)
	}

	original, err := os.ReadFile(GetRepoConfigPath(tempDir))
	if err != nil {
		t.Fatalf("failed to read scaffolded file: %v", err)
	}

	_, err = Scaffold(tempDir)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists on second Scaffold(), got %v", err)
	}

	after, err := os.ReadFile(GetRepoConfigPath(tempDir))
	if err != nil {
		t.Fatalf("failed to re-read scaffolded file: %v", err)
	}
	if string(original) != string(after) {
		t.Error("Scaffold() overwrote an existing file")
	}
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &RepoConfig{}
	cfg.PostCreate.Commands = []string{"npm install", "./setup.sh"}

	if err := cfg.Save(tempDir); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded, err := LoadRepoConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig() after Save() failed: %v", err)
	}

	if len(loaded.PostCreate.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(loaded.PostCreate.Commands))
	}
	if loaded.PostCreate.Commands[0] != "npm install" || loaded.PostCreate.Commands[1] != "./setup.sh" {
		t.Errorf("loaded commands = %v, expected %v", loaded.PostCreate.Commands, cfg.PostCreate.Commands)
	}
}
