package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ksred/ccswitch/internal/git"
	"github.com/ksred/ccswitch/internal/repoconfig"
	"github.com/ksred/ccswitch/internal/trust"
)

// setupTestRepo initializes a real git repo (with one commit) in a fresh temp
// dir and returns its path.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	run("add", "test.txt")
	run("commit", "-m", "initial commit")

	return repoDir
}

// setupTempHome isolates $HOME to a fresh temp dir so config/worktree/trust
// state from the real machine never leaks into tests.
func setupTempHome(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	return homeDir
}

// writeAndTrustRepoConfig writes .ccswitch.yaml with the given commands at
// repoDir and pre-trusts it, so CreateSession/CheckoutSession run the hooks
// without blocking on the interactive trust prompt during tests. It resolves
// repoDir through git.GetMainRepoPath, the same way Manager does internally,
// since on macOS a temp dir's canonical path (e.g. under /private) can differ
// from t.TempDir()'s reported path.
func writeAndTrustRepoConfig(t *testing.T, repoDir string, commands []string) {
	t.Helper()

	mainRepoPath, err := git.GetMainRepoPath(repoDir)
	if err != nil {
		mainRepoPath = repoDir
	}

	cfg := &repoconfig.RepoConfig{}
	cfg.PostCreate.Commands = commands
	if err := cfg.Save(mainRepoPath); err != nil {
		t.Fatalf("failed to save repo config: %v", err)
	}

	path := repoconfig.GetRepoConfigPath(mainRepoPath)
	hash, err := trust.HashFile(path)
	if err != nil {
		t.Fatalf("failed to hash repo config: %v", err)
	}

	store, err := trust.Load()
	if err != nil {
		t.Fatalf("failed to load trust store: %v", err)
	}
	store.Trust(path, hash)
	if err := store.Save(); err != nil {
		t.Fatalf("failed to save trust store: %v", err)
	}
}

func TestCreateSession_RunsPostCreateHooks(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)
	writeAndTrustRepoConfig(t, repoDir, []string{"touch marker.txt"})

	manager := NewManager(repoDir)
	if err := manager.CreateSession("my feature"); err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	sessionPath := manager.GetSessionPath("my-feature")
	if _, err := os.Stat(filepath.Join(sessionPath, "marker.txt")); err != nil {
		t.Errorf("expected post-create hook to create marker.txt in %s: %v", sessionPath, err)
	}
}

func TestCreateSession_NoRepoConfig_IsNoOp(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)

	manager := NewManager(repoDir)
	if err := manager.CreateSession("no config feature"); err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	sessionPath := manager.GetSessionPath("no-config-feature")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("expected worktree to exist at %s: %v", sessionPath, err)
	}
}

func TestCreateSession_FailingHook_DoesNotFailSession(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)
	writeAndTrustRepoConfig(t, repoDir, []string{"exit 1"})

	manager := NewManager(repoDir)
	if err := manager.CreateSession("failing hook feature"); err != nil {
		t.Fatalf("CreateSession() should not fail when a post-create hook fails, got: %v", err)
	}

	sessionPath := manager.GetSessionPath("failing-hook-feature")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("expected worktree to still exist at %s: %v", sessionPath, err)
	}
}

func TestCheckoutSession_RunsPostCreateHooks(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)
	writeAndTrustRepoConfig(t, repoDir, []string{"touch marker.txt"})

	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	manager := NewManager(repoDir)
	if err := manager.CheckoutSession("existing-branch"); err != nil {
		t.Fatalf("CheckoutSession() failed: %v", err)
	}

	sessionPath := manager.GetSessionPath("existing-branch")
	if _, err := os.Stat(filepath.Join(sessionPath, "marker.txt")); err != nil {
		t.Errorf("expected post-create hook to create marker.txt in %s: %v", sessionPath, err)
	}
}
