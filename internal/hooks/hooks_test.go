package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPostCreate_NoCommands(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCreate(nil, worktreePath, Env{})

	if outcome.Total != 0 || outcome.Ran != 0 {
		t.Errorf("expected Total=0 Ran=0, got Total=%d Ran=%d", outcome.Total, outcome.Ran)
	}
	if outcome.Failed() {
		t.Error("expected Failed() to be false with no commands")
	}
}

func TestRunPostCreate_RunsInOrder(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCreate([]string{"touch one", "touch two"}, worktreePath, Env{})

	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}
	if outcome.Ran != 2 || outcome.Total != 2 {
		t.Errorf("expected Total=2 Ran=2, got Total=%d Ran=%d", outcome.Total, outcome.Ran)
	}

	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(worktreePath, name)); err != nil {
			t.Errorf("expected file %q to exist: %v", name, err)
		}
	}
}

func TestRunPostCreate_StopsOnFailure(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCreate([]string{"touch one", "exit 1", "touch two"}, worktreePath, Env{})

	if !outcome.Failed() {
		t.Fatal("expected Failed() to be true")
	}
	if outcome.FailedCmd != "exit 1" {
		t.Errorf("expected FailedCmd = %q, got %q", "exit 1", outcome.FailedCmd)
	}
	if outcome.Ran != 2 {
		t.Errorf("expected Ran=2 (stopped after the failing command), got %d", outcome.Ran)
	}
	if outcome.Total != 3 {
		t.Errorf("expected Total=3, got %d", outcome.Total)
	}

	if _, err := os.Stat(filepath.Join(worktreePath, "one")); err != nil {
		t.Errorf("expected file %q to exist: %v", "one", err)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, "two")); !os.IsNotExist(err) {
		t.Error("expected file \"two\" to NOT exist since remaining commands should be skipped")
	}
}

func TestRunPostCreate_CwdIsWorktreePath(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCreate([]string{"touch ./marker"}, worktreePath, Env{})
	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}

	if _, err := os.Stat(filepath.Join(worktreePath, "marker")); err != nil {
		t.Errorf("expected relative path to resolve against worktreePath: %v", err)
	}
}

func TestRunPostCreate_EnvVarsInjected(t *testing.T) {
	worktreePath := t.TempDir()

	env := Env{
		WorktreePath: worktreePath,
		BranchName:   "feature/my-branch",
		SessionName:  "my-branch",
		RepoName:     "myrepo",
		RepoPath:     "/repo/path",
	}

	outcome := RunPostCreate([]string{
		`printf '%s' "$CCSWITCH_WORKTREE_PATH|$CCSWITCH_BRANCH_NAME|$CCSWITCH_SESSION_NAME|$CCSWITCH_REPO_NAME|$CCSWITCH_REPO_PATH" > env.txt`,
	}, worktreePath, env)

	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}

	data, err := os.ReadFile(filepath.Join(worktreePath, "env.txt"))
	if err != nil {
		t.Fatalf("failed to read env.txt: %v", err)
	}

	expected := strings.Join([]string{
		env.WorktreePath, env.BranchName, env.SessionName, env.RepoName, env.RepoPath,
	}, "|")
	if string(data) != expected {
		t.Errorf("env.txt = %q, expected %q", string(data), expected)
	}
}

func TestRunPostCreate_StdinNotInherited(t *testing.T) {
	worktreePath := t.TempDir()

	// If stdin were inherited from the test process (which has no data to
	// give), a command that reads from stdin should see immediate EOF rather
	// than hang, since RunPostCreate sets Stdin = nil instead of os.Stdin.
	outcome := RunPostCreate([]string{"cat > /dev/null"}, worktreePath, Env{})
	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}
}

func TestRunPostCleanup_RunsInOrder(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCleanup([]string{"touch one", "touch two"}, worktreePath, Env{})

	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}
	if outcome.Ran != 2 || outcome.Total != 2 {
		t.Errorf("expected Total=2 Ran=2, got Total=%d Ran=%d", outcome.Total, outcome.Ran)
	}

	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(worktreePath, name)); err != nil {
			t.Errorf("expected file %q to exist: %v", name, err)
		}
	}
}

func TestRunPostCleanup_StopsOnFailure(t *testing.T) {
	worktreePath := t.TempDir()

	outcome := RunPostCleanup([]string{"touch one", "exit 1", "touch two"}, worktreePath, Env{})

	if !outcome.Failed() {
		t.Fatal("expected Failed() to be true")
	}
	if outcome.FailedCmd != "exit 1" {
		t.Errorf("expected FailedCmd = %q, got %q", "exit 1", outcome.FailedCmd)
	}
	if outcome.Ran != 2 {
		t.Errorf("expected Ran=2 (stopped after the failing command), got %d", outcome.Ran)
	}

	if _, err := os.Stat(filepath.Join(worktreePath, "two")); !os.IsNotExist(err) {
		t.Error("expected file \"two\" to NOT exist since remaining commands should be skipped")
	}
}

func TestRunPostCleanup_EnvVarsInjected(t *testing.T) {
	worktreePath := t.TempDir()

	env := Env{
		WorktreePath: worktreePath,
		BranchName:   "feature/my-branch",
		SessionName:  "my-branch",
		RepoName:     "myrepo",
		RepoPath:     "/repo/path",
	}

	outcome := RunPostCleanup([]string{
		`printf '%s' "$CCSWITCH_WORKTREE_PATH|$CCSWITCH_BRANCH_NAME|$CCSWITCH_SESSION_NAME|$CCSWITCH_REPO_NAME|$CCSWITCH_REPO_PATH" > env.txt`,
	}, worktreePath, env)

	if outcome.Failed() {
		t.Fatalf("did not expect failure, got: %v", outcome.Err)
	}

	data, err := os.ReadFile(filepath.Join(worktreePath, "env.txt"))
	if err != nil {
		t.Fatalf("failed to read env.txt: %v", err)
	}

	expected := strings.Join([]string{
		env.WorktreePath, env.BranchName, env.SessionName, env.RepoName, env.RepoPath,
	}, "|")
	if string(data) != expected {
		t.Errorf("env.txt = %q, expected %q", string(data), expected)
	}
}
