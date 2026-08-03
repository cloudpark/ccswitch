// Package repoconfig loads the repo-level, git-committed ccswitch configuration
// (.ccswitch.yaml at the main repo root). Unlike the global ~/.ccswitch/config.yaml,
// this file is shared by the whole team via version control.
package repoconfig

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RepoConfig represents the contents of .ccswitch.yaml.
type RepoConfig struct {
	PostCreate PostCreateConfig `yaml:"post_create"`
	LinkShared []string         `yaml:"link_shared"`
}

// PostCreateConfig holds the commands run after a worktree is created.
type PostCreateConfig struct {
	Commands []string `yaml:"commands"`
}

// ErrAlreadyExists is returned by Scaffold when a .ccswitch.yaml already exists.
var ErrAlreadyExists = errors.New("repo config already exists")

const configFileName = ".ccswitch.yaml"

// DefaultRepoConfig returns an empty repo config (no post-create commands).
func DefaultRepoConfig() *RepoConfig {
	return &RepoConfig{}
}

// GetRepoConfigPath returns the path to the repo config file given the main repo path.
func GetRepoConfigPath(repoPath string) string {
	return filepath.Join(repoPath, configFileName)
}

// LoadRepoConfig loads .ccswitch.yaml from repoPath. If the file does not exist,
// it returns a default (empty) config with a nil error - this is a total no-op
// case, not a warning-worthy condition. If the file exists but cannot be read or
// parsed, it returns a default config together with a non-nil error so the caller
// can decide whether to warn.
func LoadRepoConfig(repoPath string) (*RepoConfig, error) {
	path := GetRepoConfigPath(repoPath)

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return DefaultRepoConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultRepoConfig(), err
	}

	cfg := DefaultRepoConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultRepoConfig(), err
	}

	return cfg, nil
}

// Save writes cfg as plain YAML (no comments) to repoPath/.ccswitch.yaml.
func (c *RepoConfig) Save(repoPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(GetRepoConfigPath(repoPath), data, 0644)
}

// Scaffold writes a starter .ccswitch.yaml (with commented documentation and an
// example) to repoPath, unless one already exists, in which case it returns
// ErrAlreadyExists and does not overwrite the file.
func Scaffold(repoPath string) (string, error) {
	path := GetRepoConfigPath(repoPath)
	if _, err := os.Stat(path); err == nil {
		return path, ErrAlreadyExists
	}
	if err := os.WriteFile(path, []byte(defaultRepoConfigTemplate), 0644); err != nil {
		return path, err
	}
	return path, nil
}

const defaultRepoConfigTemplate = `# ccswitch repo-level configuration.
# This file is committed to git and shared with your whole team.
#
# post_create.commands run automatically, in order, right after
# "ccswitch create" or "ccswitch checkout <branch>" creates a new worktree.
# Use this to set env vars, seed a database, install deps, etc.
#
# - Each entry is run as its own shell command via "sh -c".
# - Commands run with cwd set to the NEW worktree (not the main repo), so
#   relative paths/scripts resolve against the worktree.
# - Commands run with stdin closed (non-interactive) and stdout/stderr
#   streamed live to your terminal.
# - If a command exits non-zero, ccswitch prints a warning and stops running
#   the remaining commands -- it does NOT fail session creation; the worktree
#   and branch that git already created are left intact.
#
# link_shared is a list of relative paths (files or directories) that should
# be symlinked from the MAIN repo's checkout into every new worktree - e.g.
# .env files, virtualenvs, or node_modules that are slow or unwanted to
# regenerate per worktree.
#
# - Each entry is symlinked from the main repo into the same relative path
#   in the new worktree. Missing source paths are silently skipped (e.g. a
#   venv that hasn't been created yet). Existing target paths are never
#   overwritten.
# - link_shared runs BEFORE post_create.commands, so commands like
#   "npm install" can see an already-linked node_modules or .env.
# - Entries must be relative paths inside the repo (no absolute paths, no
#   "..").
#
# This file is always read from the MAIN repo's checkout, not from the
# branch being checked out into the new worktree.
#
# The first time (or after this file changes) ccswitch runs post_create
# commands or creates link_shared symlinks for you, it will ask you to
# confirm. Run "ccswitch config repo trust" to approve without being
# prompted, or "ccswitch config repo untrust" to revoke approval.
#
# Available environment variables inside each post_create command:
#   CCSWITCH_WORKTREE_PATH  - absolute path to the new worktree
#   CCSWITCH_BRANCH_NAME    - branch checked out in the worktree
#   CCSWITCH_SESSION_NAME   - slugified session name
#   CCSWITCH_REPO_NAME      - name of the main repository
#   CCSWITCH_REPO_PATH      - absolute path to the main repository
#
# link_shared:
#   - .env
#   - venv
#   - .venv
#   - frontend/node_modules
#
# post_create:
#   commands:
#     - cp "$CCSWITCH_REPO_PATH/.env.example" .env
#     - npm install
#     - ./scripts/seed-db.sh
`
