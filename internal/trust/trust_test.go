package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	return tempDir
}

func TestLoad_NoFile(t *testing.T) {
	withTempHome(t)

	store, err := Load()
	if err != nil {
		t.Fatalf("Load() with no trust file failed: %v", err)
	}
	if len(store.Trusted) != 0 {
		t.Errorf("expected empty trust store, got %v", store.Trusted)
	}
}

func TestHashFile_Deterministic(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")

	if err := os.WriteFile(path, []byte("post_create:\n  commands:\n    - echo hi\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	hash1, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile() failed: %v", err)
	}
	hash2, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile() failed: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("expected same hash for unchanged content, got %q and %q", hash1, hash2)
	}

	if err := os.WriteFile(path, []byte("post_create:\n  commands:\n    - echo changed\n"), 0644); err != nil {
		t.Fatalf("failed to rewrite file: %v", err)
	}
	hash3, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile() failed: %v", err)
	}
	if hash1 == hash3 {
		t.Error("expected different hash after content change, got the same hash")
	}
}

func TestIsTrusted(t *testing.T) {
	store := &Store{Trusted: map[string]string{}}
	path := "/repo/.ccswitch.yaml"

	if store.IsTrusted(path, "abc") {
		t.Error("expected untrusted path to report false")
	}

	store.Trust(path, "abc")
	if !store.IsTrusted(path, "abc") {
		t.Error("expected trusted path+hash to report true")
	}
	if store.IsTrusted(path, "different-hash") {
		t.Error("expected trust to require an exact hash match")
	}
}

func TestTrustAndUntrust(t *testing.T) {
	store := &Store{Trusted: map[string]string{}}
	path := "/repo/.ccswitch.yaml"

	store.Trust(path, "abc")
	if !store.IsTrusted(path, "abc") {
		t.Fatal("expected path to be trusted after Trust()")
	}

	store.Untrust(path)
	if store.IsTrusted(path, "abc") {
		t.Error("expected path to be untrusted after Untrust()")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	withTempHome(t)

	store, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	store.Trust("/repo/.ccswitch.yaml", "somehash")
	if err := store.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() failed: %v", err)
	}
	if !reloaded.IsTrusted("/repo/.ccswitch.yaml", "somehash") {
		t.Error("expected trust decision to survive Save()/Load() round trip")
	}
}
