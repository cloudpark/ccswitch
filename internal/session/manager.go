package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ksred/ccswitch/internal/config"
	"github.com/ksred/ccswitch/internal/errors"
	"github.com/ksred/ccswitch/internal/git"
	"github.com/ksred/ccswitch/internal/hooks"
	"github.com/ksred/ccswitch/internal/repoconfig"
	"github.com/ksred/ccswitch/internal/trust"
	"github.com/ksred/ccswitch/internal/ui"
	"github.com/ksred/ccswitch/internal/utils"
)

// Manager handles session operations
type Manager struct {
	worktreeManager *git.WorktreeManager
	branchManager   *git.BranchManager
	config          *config.Config
	repoPath        string
	repoName        string
}

// NewManager creates a new session manager
func NewManager(repoPath string) *Manager {
	// Get the main repository path to ensure we list all worktrees
	mainRepoPath, err := git.GetMainRepoPath(repoPath)
	if err != nil {
		// Fallback to the provided path if we can't get the main repo
		mainRepoPath = repoPath
	}

	repoName := filepath.Base(mainRepoPath)
	cfg, _ := config.Load()

	return &Manager{
		worktreeManager: git.NewWorktreeManager(mainRepoPath),
		branchManager:   git.NewBranchManager(repoPath), // Keep current path for branch operations
		config:          cfg,
		repoPath:        repoPath,
		repoName:        repoName,
	}
}

// CreateSession creates a new work session
func (m *Manager) CreateSession(description string) error {
	branchName := m.config.Branch.Prefix + utils.Slugify(description)
	sessionName := utils.Slugify(description)

	// Check if we're already on the branch we want to create
	currentBranch, err := m.branchManager.GetCurrent()
	if err == nil && currentBranch == branchName {
		return fmt.Errorf("%w: %s", errors.ErrAlreadyOnBranch, branchName)
	}

	// Check if branch already exists
	if m.branchManager.Exists(branchName) {
		return fmt.Errorf("%w: %s", errors.ErrBranchExists, branchName)
	}

	// Get worktree path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	// Get repo name from the main repo path
	mainRepoPath, err := git.GetMainRepoPath(m.repoPath)
	if err != nil {
		mainRepoPath = m.repoPath
	}
	repoName := filepath.Base(mainRepoPath)

	worktreeBasePath := filepath.Join(homeDir, ".ccswitch", "worktrees", repoName)
	worktreePath := filepath.Join(worktreeBasePath, sessionName)

	// Check if worktree directory already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("%w: %s", errors.ErrWorktreeExists, worktreePath)
	}

	// Ensure the worktree base directory exists
	if err := os.MkdirAll(worktreeBasePath, 0755); err != nil {
		return errors.Wrap(err, "failed to create worktree directory")
	}

	// Create branch
	if err := m.branchManager.Create(branchName); err != nil {
		return err
	}

	// Create worktree
	if err := m.worktreeManager.Create(worktreePath, branchName); err != nil {
		// Try to clean up the branch we just created
		_ = m.branchManager.Delete(branchName, false)
		return err
	}

	// Post-create hooks run before returning: cmd/create.go prints the "cd"
	// line for the shell wrapper only after this call succeeds, and the
	// wrapper's "grep '^cd ' | tail -1" relies on the cd line being last.
	m.runPostCreateHooks(mainRepoPath, worktreePath, branchName, sessionName)

	return nil
}

// CheckoutSession creates a worktree for an existing branch
func (m *Manager) CheckoutSession(branchName string) error {
	sessionName := utils.Slugify(branchName)

	// Check if branch exists
	if !m.branchManager.Exists(branchName) {
		return fmt.Errorf("%w: %s", errors.ErrBranchNotFound, branchName)
	}

	// Check if we're already on the branch we want to checkout
	currentBranch, err := m.branchManager.GetCurrent()
	if err == nil && currentBranch == branchName {
		return fmt.Errorf("%w: %s", errors.ErrAlreadyOnBranch, branchName)
	}

	// Get worktree path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	// Get repo name from the main repo path
	mainRepoPath, err := git.GetMainRepoPath(m.repoPath)
	if err != nil {
		mainRepoPath = m.repoPath
	}
	repoName := filepath.Base(mainRepoPath)

	worktreeBasePath := filepath.Join(homeDir, ".ccswitch", "worktrees", repoName)
	worktreePath := filepath.Join(worktreeBasePath, sessionName)

	// Check if worktree directory already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("%w: %s", errors.ErrWorktreeExists, worktreePath)
	}

	// Ensure the worktree base directory exists
	if err := os.MkdirAll(worktreeBasePath, 0755); err != nil {
		return errors.Wrap(err, "failed to create worktree directory")
	}

	// Create worktree for existing branch
	if err := m.worktreeManager.Create(worktreePath, branchName); err != nil {
		return err
	}

	// See the comment in CreateSession about hook/print ordering.
	m.runPostCreateHooks(mainRepoPath, worktreePath, branchName, sessionName)

	return nil
}

// ListSessions returns all active sessions
func (m *Manager) ListSessions() ([]git.SessionInfo, error) {
	worktrees, err := m.worktreeManager.List()
	if err != nil {
		return nil, err
	}
	return git.GetSessionsFromWorktrees(worktrees, m.repoName), nil
}

// RemoveSession removes a session and optionally its branch
func (m *Manager) RemoveSession(sessionPath string, deleteBranch bool, branchName string) error {
	// Remove worktree
	if err := m.worktreeManager.Remove(sessionPath); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Delete branch if requested
	if deleteBranch && branchName != "" {
		if err := m.branchManager.Delete(branchName, false); err != nil {
			// Check if we need to force delete
			if strings.Contains(err.Error(), "not fully merged") {
				return m.branchManager.Delete(branchName, true)
			}
			return err
		}
	}

	return nil
}

// GetSessionPath returns the path for a session
func (m *Manager) GetSessionPath(sessionName string) string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".ccswitch", "worktrees", m.repoName, sessionName)
}

// runPostCreateHooks loads the repo-level .ccswitch.yaml from mainRepoPath and
// runs any configured post_create.commands in worktreePath. This is always
// best-effort: a missing config file is a silent no-op, a malformed config file
// or a failing command prints a warning, and neither ever causes the caller
// (CreateSession/CheckoutSession) to return an error - the worktree/branch git
// already created must remain intact either way.
func (m *Manager) runPostCreateHooks(mainRepoPath, worktreePath, branchName, sessionName string) {
	repoCfg, err := repoconfig.LoadRepoConfig(mainRepoPath)
	if err != nil {
		ui.Warningf("⚠ Failed to load %s, skipping post-create hooks: %v", repoconfig.GetRepoConfigPath(mainRepoPath), err)
		return
	}
	if len(repoCfg.PostCreate.Commands) == 0 {
		return
	}

	configPath := repoconfig.GetRepoConfigPath(mainRepoPath)
	hash, err := trust.HashFile(configPath)
	if err != nil {
		ui.Warningf("⚠ Failed to read %s, skipping post-create hooks: %v", configPath, err)
		return
	}

	store, err := trust.Load()
	if err != nil {
		ui.Warningf("⚠ Failed to load trust store, skipping post-create hooks: %v", err)
		return
	}

	if !store.IsTrusted(configPath, hash) {
		if !m.promptTrust(configPath, repoCfg.PostCreate.Commands) {
			ui.Info("Skipped post-create commands (not trusted). Run 'ccswitch config repo trust' to approve them.")
			return
		}
		store.Trust(configPath, hash)
		if err := store.Save(); err != nil {
			ui.Warningf("⚠ Failed to save trust decision: %v", err)
		}
	}

	outcome := hooks.RunPostCreate(repoCfg.PostCreate.Commands, worktreePath, hooks.Env{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		SessionName:  sessionName,
		RepoName:     m.repoName,
		RepoPath:     mainRepoPath,
	})
	if outcome.Failed() {
		ui.Warningf("⚠ post-create command failed, stopping remaining commands: %s", outcome.FailedCmd)
		ui.Infof("  %v", outcome.Err)
		ui.Infof("  Ran %d/%d configured post-create command(s).", outcome.Ran, outcome.Total)
	}
}

// promptTrust asks the user to approve running the given commands, reading a
// y/N answer from stdin via utils.ReadStdinLine (shared with any earlier
// stdin prompt in this process, e.g. cmd/create.go's description prompt, so
// input isn't lost to a competing buffered reader). It returns false
// (declined) if stdin has no answer to read (e.g. closed/non-interactive
// stdin).
func (m *Manager) promptTrust(configPath string, commands []string) bool {
	ui.Warningf("⚠ %s defines post-create commands that will run automatically:", configPath)
	for _, c := range commands {
		ui.Infof("  $ %s", c)
	}
	fmt.Print("Trust and run these commands? [y/N] ")

	answer, ok := utils.ReadStdinLine()
	if !ok {
		return false
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes"
}
