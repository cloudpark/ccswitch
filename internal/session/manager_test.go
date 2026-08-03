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

// writeAndTrustRepoConfig writes .ccswitch.yaml with the given commands and
// link_shared paths at repoDir and pre-trusts it, so CreateSession/
// CheckoutSession run the hooks without blocking on the interactive trust
// prompt during tests. It resolves repoDir through git.GetMainRepoPath, the
// same way Manager does internally, since on macOS a temp dir's canonical
// path (e.g. under /private) can differ from t.TempDir()'s reported path.
func writeAndTrustRepoConfig(t *testing.T, repoDir string, commands, linkShared []string) {
	t.Helper()

	mainRepoPath, err := git.GetMainRepoPath(repoDir)
	if err != nil {
		mainRepoPath = repoDir
	}

	cfg := &repoconfig.RepoConfig{}
	cfg.PostCreate.Commands = commands
	cfg.LinkShared = linkShared
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
	writeAndTrustRepoConfig(t, repoDir, []string{"touch marker.txt"}, nil)

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
	writeAndTrustRepoConfig(t, repoDir, []string{"exit 1"}, nil)

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
	writeAndTrustRepoConfig(t, repoDir, []string{"touch marker.txt"}, nil)

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

func TestCreateSession_LinksSharedPaths(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)

	mainRepoPath, err := git.GetMainRepoPath(repoDir)
	if err != nil {
		mainRepoPath = repoDir
	}
	envSource := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(envSource, []byte("SECRET=abc"), 0644); err != nil {
		t.Fatalf("failed to write .env in main repo: %v", err)
	}

	writeAndTrustRepoConfig(t, repoDir, nil, []string{".env"})

	manager := NewManager(repoDir)
	if err := manager.CreateSession("shared env feature"); err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	sessionPath := manager.GetSessionPath("shared-env-feature")
	target := filepath.Join(sessionPath, ".env")
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("expected .env to be a symlink in the new worktree: %v", err)
	}
	if resolved != envSource {
		t.Errorf("expected .env symlink to point to %q, got %q", envSource, resolved)
	}
}

func TestCreateSession_LinkSharedRunsBeforePostCreate(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)

	mainRepoPath, err := git.GetMainRepoPath(repoDir)
	if err != nil {
		mainRepoPath = repoDir
	}
	envSource := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(envSource, []byte("SECRET=abc"), 0644); err != nil {
		t.Fatalf("failed to write .env in main repo: %v", err)
	}

	writeAndTrustRepoConfig(t, repoDir, []string{"cat .env > copied.txt"}, []string{".env"})

	manager := NewManager(repoDir)
	if err := manager.CreateSession("ordering feature"); err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	sessionPath := manager.GetSessionPath("ordering-feature")
	data, err := os.ReadFile(filepath.Join(sessionPath, "copied.txt"))
	if err != nil {
		t.Fatalf("expected copied.txt to exist: %v", err)
	}
	if string(data) != "SECRET=abc" {
		t.Errorf("expected copied.txt to contain the linked .env's contents, got %q (link_shared must run before post_create)", string(data))
	}
}

func TestCreateSession_MissingSharedSource_IsNoOp(t *testing.T) {
	setupTempHome(t)
	repoDir := setupTestRepo(t)
	writeAndTrustRepoConfig(t, repoDir, nil, []string{"nonexistent-file"})

	manager := NewManager(repoDir)
	if err := manager.CreateSession("missing shared feature"); err != nil {
		t.Fatalf("CreateSession() should not fail when a link_shared source is missing, got: %v", err)
	}

	sessionPath := manager.GetSessionPath("missing-shared-feature")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("expected worktree to still exist at %s: %v", sessionPath, err)
	}
}
