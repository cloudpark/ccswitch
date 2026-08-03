// Package linkshared symlinks repo-declared shared paths (e.g. .env, venv,
// node_modules) from the main repo into a newly created worktree. It is
// intentionally free of any printing/UI concerns - callers decide what to
// report based on the returned Outcome.
package linkshared

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Skip records a benign, non-error reason an item was not linked.
type Skip struct {
	Item   string
	Reason string
}

// Failure records an item that could not be linked due to an error.
type Failure struct {
	Item string
	Err  error
}

// Outcome describes what happened after processing a list of link_shared items.
type Outcome struct {
	Total   int // number of items configured
	Linked  int // number of symlinks actually created
	Skipped []Skip
	Errors  []Failure
}

// Failed reports whether any item failed with an error, as opposed to a benign skip.
func (o Outcome) Failed() bool {
	return len(o.Errors) > 0
}

// LinkShared symlinks each relative path in items from mainRepoPath into the
// same relative path under worktreePath. For each item:
//   - The path is cleaned and validated: absolute paths and paths that
//     traverse outside the repo root (e.g. "../x") are rejected as Errors.
//   - If the source (mainRepoPath/item) does not exist, the item is silently
//     skipped - shared paths like .venv or node_modules are often
//     environment-dependent and may not exist yet.
//   - If the target (worktreePath/item) already exists - checked with Lstat,
//     so a dangling symlink counts as "exists" - the item is skipped, so
//     repeated runs are idempotent and never clobber something already there.
//   - Otherwise, the target's parent directory is created and a symlink from
//     worktreePath/item to mainRepoPath/item is created.
//
// LinkShared never returns an error itself; every per-item problem is
// recorded in the returned Outcome so the caller can report a non-fatal
// warning without failing session creation.
func LinkShared(items []string, mainRepoPath, worktreePath string) Outcome {
	outcome := Outcome{Total: len(items)}

	for _, item := range items {
		cleaned := filepath.Clean(item)

		if cleaned == "." || cleaned == "" ||
			filepath.IsAbs(cleaned) ||
			cleaned == ".." ||
			strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
			outcome.Errors = append(outcome.Errors, Failure{
				Item: item,
				Err:  errors.New("invalid link_shared entry: must be a relative path inside the repo"),
			})
			continue
		}

		source := filepath.Join(mainRepoPath, cleaned)
		target := filepath.Join(worktreePath, cleaned)

		if _, err := os.Stat(source); err != nil {
			outcome.Skipped = append(outcome.Skipped, Skip{Item: item, Reason: "source does not exist"})
			continue
		}

		if _, err := os.Lstat(target); err == nil {
			outcome.Skipped = append(outcome.Skipped, Skip{Item: item, Reason: "target already exists"})
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			outcome.Errors = append(outcome.Errors, Failure{Item: item, Err: err})
			continue
		}

		if err := os.Symlink(source, target); err != nil {
			outcome.Errors = append(outcome.Errors, Failure{Item: item, Err: err})
			continue
		}

		outcome.Linked++
	}

	return outcome
}
