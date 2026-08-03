// Package hooks runs the shell commands configured by a repo's post-create
// config. It is intentionally free of any printing/UI concerns - callers
// decide what to report based on the returned Outcome.
package hooks

import (
	"os"
	"os/exec"
)

// Env holds the CCSWITCH_* values injected into each hook command's environment.
type Env struct {
	WorktreePath string
	BranchName   string
	SessionName  string
	RepoName     string
	RepoPath     string
}

// Outcome describes what happened after running a list of post-create commands.
type Outcome struct {
	Total     int    // number of commands configured
	Ran       int    // number of commands actually started before stopping
	FailedCmd string // the command that failed; empty if none failed
	Err       error  // error from the failed command; nil if none failed
}

// Failed reports whether a command failed and the remaining commands were skipped.
func (o Outcome) Failed() bool {
	return o.Err != nil
}

// RunPostCreate runs each command in commands sequentially via "sh -c <command>",
// with cwd set to worktreePath and env = os.Environ() plus CCSWITCH_* vars derived
// from env. stdin is not inherited so a hook that reads stdin can't block the
// terminal. stdout/stderr are inherited and streamed live. Execution stops at the
// first command that exits non-zero; RunPostCreate itself never returns an error -
// failures are reported via the returned Outcome so the caller can print a
// non-fatal warning without failing the parent command.
func RunPostCreate(commands []string, worktreePath string, env Env) Outcome {
	outcome := Outcome{Total: len(commands)}
	if len(commands) == 0 {
		return outcome
	}

	cmdEnv := append(os.Environ(),
		"CCSWITCH_WORKTREE_PATH="+env.WorktreePath,
		"CCSWITCH_BRANCH_NAME="+env.BranchName,
		"CCSWITCH_SESSION_NAME="+env.SessionName,
		"CCSWITCH_REPO_NAME="+env.RepoName,
		"CCSWITCH_REPO_PATH="+env.RepoPath,
	)

	for _, command := range commands {
		c := exec.Command("sh", "-c", command)
		c.Dir = worktreePath
		c.Env = cmdEnv
		c.Stdin = nil
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		outcome.Ran++
		if err := c.Run(); err != nil {
			outcome.FailedCmd = command
			outcome.Err = err
			return outcome
		}
	}

	return outcome
}
