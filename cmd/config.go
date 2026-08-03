package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ksred/ccswitch/internal/config"
	"github.com/ksred/ccswitch/internal/git"
	"github.com/ksred/ccswitch/internal/repoconfig"
	"github.com/ksred/ccswitch/internal/trust"
	"github.com/ksred/ccswitch/internal/ui"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show ccswitch configuration",
		Run:   showConfig,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show config file path",
		Run:   showConfigPath,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		Run:   initConfig,
	})

	cmd.AddCommand(newConfigRepoCmd())

	return cmd
}

func showConfig(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		ui.Errorf("✗ Failed to load config: %v", err)
		return
	}

	ui.Title("⚙️  ccswitch Configuration")
	fmt.Println()

	ui.Success("Branch:")
	ui.Infof("  Prefix: %s", cfg.Branch.Prefix)
	fmt.Println()

	ui.Success("Worktree:")
	ui.Infof("  Relative path: %s", cfg.Worktree.RelativePath)
	fmt.Println()

	ui.Success("UI:")
	ui.Infof("  Show emoji: %v", cfg.UI.ShowEmoji)
	ui.Infof("  Color scheme: %s", cfg.UI.ColorScheme)
	fmt.Println()

	ui.Success("Git:")
	ui.Infof("  Default branch: %s", cfg.Git.DefaultBranch)
	ui.Infof("  Auto fetch: %v", cfg.Git.AutoFetch)
	fmt.Println()

	configPath := config.GetConfigPath()
	ui.Infof("Config file: %s", configPath)
}

func showConfigPath(cmd *cobra.Command, args []string) {
	fmt.Println(config.GetConfigPath())
}

func initConfig(cmd *cobra.Command, args []string) {
	cfg := config.DefaultConfig()
	if err := cfg.Save(); err != nil {
		ui.Errorf("✗ Failed to create config: %v", err)
		return
	}

	configPath := config.GetConfigPath()
	ui.Successf("✓ Created default config at: %s", configPath)
	fmt.Println()
	fmt.Println("You can now edit this file to customize ccswitch behavior.")
}

func newConfigRepoCmd() *cobra.Command {
	repoCmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage the repo-level ccswitch configuration (.ccswitch.yaml)",
		Run:   showRepoConfig,
	}

	repoCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show repo-level configuration",
		Run:   showRepoConfig,
	})

	repoCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show repo config file path",
		Run:   showRepoConfigPath,
	})

	repoCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create a starter .ccswitch.yaml in the repo root",
		Run:   initRepoConfig,
	})

	repoCmd.AddCommand(&cobra.Command{
		Use:   "trust",
		Short: "Trust this repo's .ccswitch.yaml so post-create commands run without prompting",
		Run:   trustRepoConfig,
	})

	repoCmd.AddCommand(&cobra.Command{
		Use:   "untrust",
		Short: "Revoke trust for this repo's .ccswitch.yaml",
		Run:   untrustRepoConfig,
	})

	return repoCmd
}

// resolveMainRepoPath returns the main repo path for the current working
// directory, falling back to the current directory if it can't be resolved
// (e.g. not inside a git worktree).
func resolveMainRepoPath() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	mainRepoPath, err := git.GetMainRepoPath(currentDir)
	if err != nil {
		mainRepoPath = currentDir
	}
	return mainRepoPath, nil
}

func showRepoConfig(cmd *cobra.Command, args []string) {
	mainRepoPath, err := resolveMainRepoPath()
	if err != nil {
		ui.Errorf("✗ Failed to resolve repo path: %v", err)
		return
	}

	cfg, err := repoconfig.LoadRepoConfig(mainRepoPath)
	if err != nil {
		ui.Errorf("✗ Failed to load repo config: %v", err)
		return
	}

	ui.Title("⚙️  ccswitch Repo Configuration")
	fmt.Println()

	if len(cfg.PostCreate.Commands) == 0 {
		ui.Info("No post-create commands configured.")
	} else {
		ui.Success("Post-create commands:")
		for i, c := range cfg.PostCreate.Commands {
			ui.Infof("  %d. %s", i+1, c)
		}

		configPath := repoconfig.GetRepoConfigPath(mainRepoPath)
		trusted := false
		if hash, hashErr := trust.HashFile(configPath); hashErr == nil {
			if store, loadErr := trust.Load(); loadErr == nil {
				trusted = store.IsTrusted(configPath, hash)
			}
		}
		fmt.Println()
		if trusted {
			ui.Info("Trusted: yes")
		} else {
			ui.Info("Trusted: no (will prompt before running)")
		}
	}
	fmt.Println()

	path := repoconfig.GetRepoConfigPath(mainRepoPath)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		ui.Infof("Repo config file: %s (not found; run 'ccswitch config repo init' to create one)", path)
	} else {
		ui.Infof("Repo config file: %s", path)
	}
}

func showRepoConfigPath(cmd *cobra.Command, args []string) {
	mainRepoPath, err := resolveMainRepoPath()
	if err != nil {
		ui.Errorf("✗ Failed to resolve repo path: %v", err)
		return
	}
	fmt.Println(repoconfig.GetRepoConfigPath(mainRepoPath))
}

func initRepoConfig(cmd *cobra.Command, args []string) {
	mainRepoPath, err := resolveMainRepoPath()
	if err != nil {
		ui.Errorf("✗ Failed to resolve repo path: %v", err)
		return
	}

	path, err := repoconfig.Scaffold(mainRepoPath)
	if err != nil {
		if errors.Is(err, repoconfig.ErrAlreadyExists) {
			ui.Warningf("⚠ %s already exists, not overwriting.", path)
			return
		}
		ui.Errorf("✗ Failed to create repo config: %v", err)
		return
	}

	ui.Successf("✓ Created repo config at: %s", path)
	fmt.Println()
	fmt.Println("Edit this file to add post-create commands, then commit it so your team shares the same setup.")
}

func trustRepoConfig(cmd *cobra.Command, args []string) {
	mainRepoPath, err := resolveMainRepoPath()
	if err != nil {
		ui.Errorf("✗ Failed to resolve repo path: %v", err)
		return
	}

	path := repoconfig.GetRepoConfigPath(mainRepoPath)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		ui.Errorf("✗ No %s found. Run 'ccswitch config repo init' first.", path)
		return
	}

	cfg, err := repoconfig.LoadRepoConfig(mainRepoPath)
	if err != nil {
		ui.Errorf("✗ Failed to load repo config: %v", err)
		return
	}

	hash, err := trust.HashFile(path)
	if err != nil {
		ui.Errorf("✗ Failed to read %s: %v", path, err)
		return
	}

	store, err := trust.Load()
	if err != nil {
		ui.Errorf("✗ Failed to load trust store: %v", err)
		return
	}

	store.Trust(path, hash)
	if err := store.Save(); err != nil {
		ui.Errorf("✗ Failed to save trust decision: %v", err)
		return
	}

	ui.Successf("✓ Trusted %s", path)
	if len(cfg.PostCreate.Commands) > 0 {
		fmt.Println()
		ui.Info("These commands will now run without prompting:")
		for i, c := range cfg.PostCreate.Commands {
			ui.Infof("  %d. %s", i+1, c)
		}
	}
}

func untrustRepoConfig(cmd *cobra.Command, args []string) {
	mainRepoPath, err := resolveMainRepoPath()
	if err != nil {
		ui.Errorf("✗ Failed to resolve repo path: %v", err)
		return
	}

	path := repoconfig.GetRepoConfigPath(mainRepoPath)

	store, err := trust.Load()
	if err != nil {
		ui.Errorf("✗ Failed to load trust store: %v", err)
		return
	}

	store.Untrust(path)
	if err := store.Save(); err != nil {
		ui.Errorf("✗ Failed to save trust decision: %v", err)
		return
	}

	ui.Successf("✓ Revoked trust for %s", path)
}
