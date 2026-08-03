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
	"github.com/ksred/ccswitch/internal/linkshared"
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

	// Post-create hooks and shared-path linking run before returning:
	// cmd/create.go prints the "cd" line for the shell wrapper only after
	// this call succeeds, and the wrapper's "grep '^cd ' | tail -1" relies
	// on the cd line being last.
	m.runPostCreateAndLinkShared(mainRepoPath, worktreePath, branchName, sessionName)

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
	m.runPostCreateAndLinkShared(mainRepoPath, worktreePath, branchName, sessionName)

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
	// Post-remove hooks run first, while sessionPath still exists, so cleanup
	// commands can reference files inside the worktree being torn down.
	mainRepoPath, err := git.GetMainRepoPath(m.repoPath)
	if err != nil {
		mainRepoPath = m.repoPath
	}
	sessionName := filepath.Base(sessionPath)
	m.runPostRemove(mainRepoPath, sessionPath, branchName, sessionName)

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

// runPostCreateAndLinkShared loads the repo-level .ccswitch.yaml from
// mainRepoPath and, if trusted, symlinks any configured link_shared paths
// into worktreePath before running any configured post_create.commands
// there. Linking runs first so that post_create commands can see already
// materialized shared state (e.g. a shared node_modules or .env). This is
// always best-effort: a missing config file is a silent no-op, and a
// malformed config file, a failing command, or a failed link only prints a
// warning - none of these ever cause the caller (CreateSession/
// CheckoutSession) to return an error; the worktree/branch git already
// created must remain intact either way.
func (m *Manager) runPostCreateAndLinkShared(mainRepoPath, worktreePath, branchName, sessionName string) {
	repoCfg, err := repoconfig.LoadRepoConfig(mainRepoPath)
	if err != nil {
		ui.Warningf("⚠ Failed to load %s, skipping post-create hooks and shared links: %v", repoconfig.GetRepoConfigPath(mainRepoPath), err)
		return
	}
	if len(repoCfg.PostCreate.Commands) == 0 && len(repoCfg.LinkShared) == 0 {
		return
	}

	configPath := repoconfig.GetRepoConfigPath(mainRepoPath)
	hash, err := trust.HashFile(configPath)
	if err != nil {
		ui.Warningf("⚠ Failed to read %s, skipping post-create hooks and shared links: %v", configPath, err)
		return
	}

	store, err := trust.Load()
	if err != nil {
		ui.Warningf("⚠ Failed to load trust store, skipping post-create hooks and shared links: %v", err)
		return
	}

	if !store.IsTrusted(configPath, hash) {
		if !m.promptTrust(configPath, repoCfg.PostCreate.Commands, repoCfg.LinkShared) {
			ui.Info("Skipped post-create commands and shared-path linking (not trusted). Run 'ccswitch config repo trust' to approve them.")
			return
		}
		store.Trust(configPath, hash)
		if err := store.Save(); err != nil {
			ui.Warningf("⚠ Failed to save trust decision: %v", err)
		}
	}

	if len(repoCfg.LinkShared) > 0 {
		linkOutcome := linkshared.LinkShared(repoCfg.LinkShared, mainRepoPath, worktreePath)
		if linkOutcome.Linked > 0 {
			ui.Infof("🔗 Linked %d/%d shared path(s) into the new worktree.", linkOutcome.Linked, linkOutcome.Total)
		}
		for _, f := range linkOutcome.Errors {
			ui.Warningf("⚠ Failed to link shared path %q: %v", f.Item, f.Err)
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

// runPostRemove loads the repo-level .ccswitch.yaml from mainRepoPath and, if
// trusted, runs any configured post_remove.commands with cwd set to
// sessionPath. It must be called before the worktree at sessionPath is
// removed, since that's what gives the commands a cwd to run in. Like
// runPostCreateAndLinkShared, this is always best-effort: a missing config
// file or no post_remove commands is a silent no-op, and a malformed config
// file or a failing command only prints a warning - it never blocks
// RemoveSession from removing the worktree/branch.
func (m *Manager) runPostRemove(mainRepoPath, sessionPath, branchName, sessionName string) {
	repoCfg, err := repoconfig.LoadRepoConfig(mainRepoPath)
	if err != nil {
		ui.Warningf("⚠ Failed to load %s, skipping post-remove hooks: %v", repoconfig.GetRepoConfigPath(mainRepoPath), err)
		return
	}
	if len(repoCfg.PostRemove.Commands) == 0 {
		return
	}

	configPath := repoconfig.GetRepoConfigPath(mainRepoPath)
	hash, err := trust.HashFile(configPath)
	if err != nil {
		ui.Warningf("⚠ Failed to read %s, skipping post-remove hooks: %v", configPath, err)
		return
	}

	store, err := trust.Load()
	if err != nil {
		ui.Warningf("⚠ Failed to load trust store, skipping post-remove hooks: %v", err)
		return
	}

	if !store.IsTrusted(configPath, hash) {
		if !m.promptTrust(configPath, repoCfg.PostRemove.Commands, nil) {
			ui.Info("Skipped post-remove commands (not trusted). Run 'ccswitch config repo trust' to approve them.")
			return
		}
		store.Trust(configPath, hash)
		if err := store.Save(); err != nil {
			ui.Warningf("⚠ Failed to save trust decision: %v", err)
		}
	}

	outcome := hooks.RunPostRemove(repoCfg.PostRemove.Commands, sessionPath, hooks.Env{
		WorktreePath: sessionPath,
		BranchName:   branchName,
		SessionName:  sessionName,
		RepoName:     m.repoName,
		RepoPath:     mainRepoPath,
	})
	if outcome.Failed() {
		ui.Warningf("⚠ post-remove command failed, stopping remaining commands: %s", outcome.FailedCmd)
		ui.Infof("  %v", outcome.Err)
		ui.Infof("  Ran %d/%d configured post-remove command(s).", outcome.Ran, outcome.Total)
	}
}

// promptTrust asks the user to approve running the given commands and/or
// creating the given shared-path symlinks, reading a y/N answer from stdin
// via utils.ReadStdinLine (shared with any earlier stdin prompt in this
// process, e.g. cmd/create.go's description prompt, so input isn't lost to
// a competing buffered reader). Declining skips BOTH post_create.commands
// and link_shared - trust is all-or-nothing per file. It returns false
// (declined) if stdin has no answer to read (e.g. closed/non-interactive
// stdin).
func (m *Manager) promptTrust(configPath string, commands, linkShared []string) bool {
	ui.Warningf("⚠ %s defines actions that will run automatically:", configPath)
	if len(commands) > 0 {
		ui.Info("Commands to run:")
		for _, c := range commands {
			ui.Infof("  $ %s", c)
		}
	}
	if len(linkShared) > 0 {
		ui.Info("Paths to symlink from the main repo into the new worktree:")
		for _, p := range linkShared {
			ui.Infof("  %s", p)
		}
	}
	if len(linkShared) > 0 {
		fmt.Print("Trust and run/link these? [y/N] ")
	} else {
		fmt.Print("Trust and run these? [y/N] ")
	}

	answer, ok := utils.ReadStdinLine()
	if !ok {
		return false
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes"
}
