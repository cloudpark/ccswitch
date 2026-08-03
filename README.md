# 🔀 ccswitch

Built by [Kyle Redelinghuys](https://ksred.com)

A friendly CLI tool for managing multiple git worktrees, perfect for juggling different features, experiments, or Claude Code sessions without the context-switching headaches.

## 🎯 What is this?

`ccswitch` helps you create and manage git worktrees with a clean, intuitive interface. Each worktree gets its own directory, letting you work on multiple features simultaneously without stashing changes or switching branches in place.

## ✨ Features

- **🚀 Quick Session Creation** - Describe what you're working on, get a branch and worktree instantly
- **📋 Interactive Session List** - See all your active work sessions with a clean TUI
- **🧹 Smart Cleanup** - Remove worktrees and optionally delete branches when done
- **🗑️ Bulk Cleanup** - Remove ALL worktrees at once with `cleanup --all` (perfect for spring cleaning!)
- **🐚 Shell Integration** - Automatically `cd` into new worktrees (no copy-pasting paths!)
- **🎨 Pretty Output** - Color-coded messages and clean formatting
- **⚙️ Post-Create Hooks** - Run a repo-shared setup script automatically for every new worktree

## 📦 Installation

### Using Make
```bash
# Clone the repo
git clone https://github.com/ksred/ccswitch.git
cd ccswitch

# Build and install
make install

# Add shell integration to your .bashrc or .zshrc
echo 'eval "$(ccswitch shell-init)"' >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc                                    # or ~/.zshrc
```

### Manual Installation
```bash
# Build the binary
go build -o ccswitch .

# Move to your PATH
sudo mv ccswitch /usr/local/bin/

# Add shell integration
echo 'eval "$(ccswitch shell-init)"' >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc                                    # or ~/.zshrc
```

## 🚀 Usage

### Create a New Work Session
```bash
ccswitch
# 🚀 What are you working on? Fix authentication bug
# ✓ Created session: fix-authentication-bug
#   Branch: feature/fix-authentication-bug
#   Location: ~/.ccswitch/worktrees/my-project/fix-authentication-bug
#
# Automatically switches to the new directory!
```

### Check Out an Existing Branch
```bash
ccswitch checkout feature/existing-branch
# ✓ Checked out session: existing-branch
#   Branch: feature/existing-branch
#   Location: ~/.ccswitch/worktrees/my-project/existing-branch
#
# Automatically switches to the new directory!
```

### List Active Sessions
```bash
ccswitch list
# Shows an interactive list of all your worktrees
# Use arrow keys to navigate, Enter to select, q to quit
```

### Switch Between Sessions
```bash
ccswitch switch
# Interactive selection of session to switch to

ccswitch switch fix-auth-bug
# Direct switch to a specific session
# Automatically changes to the session directory!
```

### Clean Up When Done
```bash
ccswitch cleanup
# Select a session interactively, or:

ccswitch cleanup fix-authentication-bug
# Delete branch feature/fix-authentication-bug? (y/N): y
# ✓ Removed session and branch: fix-authentication-bug

# Bulk cleanup - remove ALL worktrees at once!
ccswitch cleanup --all
# ⚠️  You are about to remove the following worktrees:
#   • feature-1 (feature/feature-1)
#   • feature-2 (feature/feature-2)
#   • bugfix-1 (feature/bugfix-1)
# Press Enter to continue or Ctrl+C to cancel...
# Delete associated branches as well? (y/N): y
# ✓ Successfully removed: feature-1
# ✓ Successfully removed: feature-2
# ✓ Successfully removed: bugfix-1
# ✅ All 3 worktrees removed successfully!
# ✓ Switched to main branch
```

### Post-Create Hooks (env setup, DB seeding, etc.)

Share a setup script with your team so every new worktree is ready to develop in immediately - set env vars, install deps, clone/seed a dev database, and so on.

```bash
# Scaffold a starter .ccswitch.yaml at the repo root
ccswitch config repo init

# Edit it to add your commands, then commit it so the whole team gets it
ccswitch config repo show
```

`.ccswitch.yaml`:
```yaml
post_create:
  commands:
    - cp "$CCSWITCH_REPO_PATH/.env.example" .env
    - npm install
    - ./scripts/seed-db.sh
```

- Commands run in order via `sh -c`, with `cwd` set to the **new worktree**.
- The first time (or whenever the file's contents change), ccswitch prints the commands and asks you to confirm before running them - approval is remembered per-repo. Manage this with `ccswitch config repo trust` / `ccswitch config repo untrust`.
- If a command fails, ccswitch prints a warning and skips the rest, but the session is still created.
- Available environment variables: `CCSWITCH_WORKTREE_PATH`, `CCSWITCH_BRANCH_NAME`, `CCSWITCH_SESSION_NAME`, `CCSWITCH_REPO_NAME`, `CCSWITCH_REPO_PATH`.

## 🛠️ Development

### Quick Start
```bash
# Run directly
make run

# Run tests
make test

# See all commands
make help
```

### Testing
```bash
# Unit tests only (fast, no git required)
make test-unit

# All tests including integration
make test

# Run tests in Docker (clean environment)
make test-docker

# Generate coverage report
make coverage
```

### Project Structure
```
ccswitch/
├── main.go              # Entry point
├── cmd/                 # CLI commands (create, checkout, list, switch, cleanup, config, ...)
├── internal/            # Core packages (session, git, config, repoconfig, trust, hooks, ui, utils)
├── Makefile            # Build automation
├── *_test.go            # Test files (one per package)
├── Dockerfile.test     # Docker test environment
└── README.md           # You are here! 👋
```

## 🤔 How It Works

1. **Session Creation**: Converts your description into a branch name (e.g., "Fix login bug" → `feature/fix-login-bug`)
2. **Centralized Storage**: Creates worktrees in `~/.ccswitch/worktrees/repo-name/session-name` - your projects stay clean!
3. **Automatic Navigation**: The bash wrapper captures the output and `cd`s you into the new directory
4. **Session Tracking**: Lists all worktrees except the main one as active sessions

### Directory Structure
```
~/.ccswitch/                      # All ccswitch data in your home directory
└── worktrees/                    # Centralized worktree storage
    ├── my-project/               # Organized by repository name
    │   ├── fix-login-bug/        # Individual sessions
    │   ├── add-new-feature/
    │   └── refactor-ui/
    └── another-project/
        ├── update-deps/
        └── new-feature/

# Your project directories remain completely clean!
/Users/you/projects/
├── my-project/                   # Just your main repository
└── another-project/              # No clutter!
```

## 🔧 Requirements

- **Go** 1.23.4 or higher (for building)
- **Git** 2.20 or higher (for worktree support)
- **Bash** or **Zsh** (for shell integration)

## 💡 Tips

- Use descriptive session names - they become your branch names!
- Regular cleanup keeps your workspace tidy
- Each worktree is independent - perfect for testing different approaches
- The tool respects your current branch when creating new sessions

## 🐛 Troubleshooting

**"Failed to create worktree"**
- Check if the branch already exists: `git branch -a`
- Ensure you're in a git repository
- Verify you have write permissions in the parent directory

**Shell integration not working**
- Make sure you've added `eval "$(ccswitch shell-init)"` to your `~/.bashrc` or `~/.zshrc` and reloaded your shell (`source ~/.bashrc`/`source ~/.zshrc`)
- Check that `ccswitch` is in your PATH
- Try using the full path: `/usr/local/bin/ccswitch`

## 📝 License

MIT License - feel free to use this in your projects!

## 🤝 Contributing

Found a bug? Have an idea? Feel free to open an issue or submit a PR!

---

Made with ❤️ for developers who juggle multiple features at once
