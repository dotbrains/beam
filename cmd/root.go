package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dotbrains/beam/internal/config"
	"github.com/spf13/cobra"
)

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "beam",
		Short: "Webhook notifications and interactive approvals",
		Long:  "Beam turns webhook requests into notification events and provides a CLI for scripted notifications, prompts, and live activity state.",
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		Version: version,
	}

	root.SetVersionTemplate(fmt.Sprintf("beam version %s\n", version))

	// Subcommands
	root.AddCommand(newConfigCmd(), newServeCmd(), newNotifyCmd(), newAskCmd(), newInteractionCmd(), newActivityCmd(), newServicesCmd())

	return root
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	var force bool

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.ConfigPath()
			if err != nil {
				return err
			}

			if !force {
				if _, err := os.Stat(cfgPath); err == nil {
					return fmt.Errorf("config already exists at %s (use --force to overwrite)", cfgPath)
				}
			}

			if err := config.Save(config.DefaultConfig()); err != nil {
				return err
			}

			// Shorten the path for display.
			display := cfgPath
			if home, err := os.UserHomeDir(); err == nil {
				if rel, err := filepath.Rel(home, cfgPath); err == nil {
					display = "~/" + rel
				}
			}

			cmd.Printf("✓ Wrote default config to %s\nEdit the file to customize settings.\n", display)
			return nil
		},
	}
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")

	cmd.AddCommand(initCmd)
	return cmd
}

// Execute runs the root command.
func Execute(version string) error {
	return newRootCmd(version).Execute()
}
