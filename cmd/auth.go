package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dotbrains/beam/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage local CLI credentials",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthStatusCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var token, clientName string
	var scopes []string
	var expiresIn time.Duration
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store local CLI credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			token = firstNonEmpty(token, os.Getenv("BEAM_TOKEN"))
			if token == "" {
				return UsageError{Err: fmt.Errorf("pass --token or set BEAM_TOKEN")}
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.Token = token
			cfg.Scopes = scopes
			cfg.ClientName = clientName
			cfg.ExpiresAt = ""
			if expiresIn > 0 {
				cfg.ExpiresAt = time.Now().UTC().Add(expiresIn).Format(time.RFC3339)
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeAuthStatus(cmd, cfg, "config")
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "webhook or agent token to store")
	cmd.Flags().StringArrayVar(&scopes, "scope", nil, "credential scope (repeatable)")
	cmd.Flags().StringVar(&clientName, "client-name", "", "human-readable client name")
	cmd.Flags().DurationVar(&expiresIn, "expires-in", 0, "credential lifetime")
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			source := "config"
			if token := os.Getenv("BEAM_TOKEN"); token != "" {
				cfg.Token = token
				source = "env"
			}
			return writeAuthStatus(cmd, cfg, source)
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove local CLI credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.Token = ""
			cfg.Scopes = nil
			cfg.ClientName = ""
			cfg.ExpiresAt = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeAuthStatus(cmd, cfg, "config")
		},
	}
}

func writeAuthStatus(cmd *cobra.Command, cfg *config.Config, source string) error {
	expired := false
	if cfg.ExpiresAt != "" {
		if expiresAt, err := time.Parse(time.RFC3339, cfg.ExpiresAt); err == nil {
			expired = time.Now().UTC().After(expiresAt)
		}
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
		"ok":            true,
		"authenticated": cfg.Token != "" && !expired,
		"apiUrl":        cfg.APIURL,
		"credential": map[string]any{
			"source":     source,
			"configured": cfg.Token != "",
			"clientName": cfg.ClientName,
			"scopes":     cfg.Scopes,
			"expiresAt":  cfg.ExpiresAt,
			"expired":    expired,
		},
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
