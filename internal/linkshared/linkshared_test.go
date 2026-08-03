package linkshared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkShared_NoItems(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	outcome := LinkShared(nil, mainRepoPath, worktreePath)

	if outcome.Total != 0 || outcome.Linked != 0 {
		t.Errorf("expected Total=0 Linked=0, got Total=%d Linked=%d", outcome.Total, outcome.Linked)
	}
	if outcome.Failed() {
		t.Error("expected Failed() to be false with no items")
	}
}

func TestLinkShared_FileSource(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(source, []byte("SECRET=abc"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	outcome := LinkShared([]string{".env"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure: %+v", outcome.Errors)
	}
	if outcome.Linked != 1 {
		t.Fatalf("expected Linked=1, got %d", outcome.Linked)
	}

	target := filepath.Join(worktreePath, ".env")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("expected target to exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected target to be a symlink, mode=%v", info.Mode())
	}

	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	if resolved != source {
		t.Errorf("expected symlink to point to %q, got %q", source, resolved)
	}
}

func TestLinkShared_DirSource(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, "venv")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	outcome := LinkShared([]string{"venv"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure: %+v", outcome.Errors)
	}
	if outcome.Linked != 1 {
		t.Fatalf("expected Linked=1, got %d", outcome.Linked)
	}

	target := filepath.Join(worktreePath, "venv")
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("expected target to be a symlink: %v", err)
	}
	if resolved != source {
		t.Errorf("expected symlink to point to %q, got %q", source, resolved)
	}
}

func TestLinkShared_NestedPath_CreatesParentDir(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, "frontend", "node_modules")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	outcome := LinkShared([]string{"frontend/node_modules"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure: %+v", outcome.Errors)
	}
	if outcome.Linked != 1 {
		t.Fatalf("expected Linked=1, got %d", outcome.Linked)
	}

	target := filepath.Join(worktreePath, "frontend", "node_modules")
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("expected target to be a symlink: %v", err)
	}
	if resolved != source {
		t.Errorf("expected symlink to point to %q, got %q", source, resolved)
	}
}

func TestLinkShared_MissingSource_IsSilentSkip(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	outcome := LinkShared([]string{"does-not-exist"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure: %+v", outcome.Errors)
	}
	if outcome.Linked != 0 {
		t.Errorf("expected Linked=0, got %d", outcome.Linked)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0].Reason != "source does not exist" {
		t.Errorf("expected one skip with reason %q, got %+v", "source does not exist", outcome.Skipped)
	}

	if _, err := os.Lstat(filepath.Join(worktreePath, "does-not-exist")); !os.IsNotExist(err) {
		t.Error("expected no target to be created")
	}
}

func TestLinkShared_ExistingTargetFile_IsSkipped(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(source, []byte("SECRET=main"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	target := filepath.Join(worktreePath, ".env")
	if err := os.WriteFile(target, []byte("SECRET=worktree"), 0644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	outcome := LinkShared([]string{".env"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure: %+v", outcome.Errors)
	}
	if outcome.Linked != 0 {
		t.Errorf("expected Linked=0, got %d", outcome.Linked)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0].Reason != "target already exists" {
		t.Errorf("expected one skip with reason %q, got %+v", "target already exists", outcome.Skipped)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read target: %v", err)
	}
	if string(data) != "SECRET=worktree" {
		t.Errorf("expected existing target to be untouched, got %q", string(data))
	}
}

func TestLinkShared_ExistingTargetDanglingSymlink_IsSkipped(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(source, []byte("SECRET=main"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	target := filepath.Join(worktreePath, ".env")
	if err := os.Symlink(filepath.Join(worktreePath, "nonexistent-target"), target); err != nil {
		t.Fatalf("failed to create dangling symlink: %v", err)
	}

	// Sanity check: os.Stat on a dangling symlink reports not-exist.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected Stat on dangling symlink to report not-exist, got err=%v", err)
	}

	outcome := LinkShared([]string{".env"}, mainRepoPath, worktreePath)

	if outcome.Failed() {
		t.Fatalf("did not expect failure (should skip via Lstat, not error): %+v", outcome.Errors)
	}
	if len(outcome.Skipped) != 1 {
		t.Errorf("expected one skip, got %+v", outcome.Skipped)
	}

	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("expected target to still be the original dangling symlink: %v", err)
	}
	if resolved == source {
		t.Error("expected the dangling symlink to be left untouched, not replaced")
	}
}

func TestLinkShared_AbsolutePath_IsRejected(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	outcome := LinkShared([]string{"/etc/passwd"}, mainRepoPath, worktreePath)

	if !outcome.Failed() {
		t.Fatal("expected Failed() to be true for an absolute path entry")
	}
	if len(outcome.Errors) != 1 {
		t.Fatalf("expected one error, got %+v", outcome.Errors)
	}
}

func TestLinkShared_ParentTraversal_IsRejected(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	items := []string{"..", "../outside", "a/../../b"}
	outcome := LinkShared(items, mainRepoPath, worktreePath)

	if !outcome.Failed() {
		t.Fatal("expected Failed() to be true")
	}
	if len(outcome.Errors) != len(items) {
		t.Errorf("expected %d errors, got %d: %+v", len(items), len(outcome.Errors), outcome.Errors)
	}
}

func TestLinkShared_MultipleItems_MixedResults(t *testing.T) {
	mainRepoPath := t.TempDir()
	worktreePath := t.TempDir()

	source := filepath.Join(mainRepoPath, ".env")
	if err := os.WriteFile(source, []byte("SECRET=abc"), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	outcome := LinkShared([]string{".env", "missing-thing", "../escape"}, mainRepoPath, worktreePath)

	if outcome.Total != 3 {
		t.Errorf("expected Total=3, got %d", outcome.Total)
	}
	if outcome.Linked != 1 {
		t.Errorf("expected Linked=1, got %d", outcome.Linked)
	}
	if len(outcome.Skipped) != 1 {
		t.Errorf("expected 1 skip, got %d: %+v", len(outcome.Skipped), outcome.Skipped)
	}
	if len(outcome.Errors) != 1 {
		t.Errorf("expected 1 error, got %d: %+v", len(outcome.Errors), outcome.Errors)
	}
}
