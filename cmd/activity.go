package cmd

import (
	"encoding/json"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/spf13/cobra"
)

func newActivityCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "activity", Short: "Drive Live Activity state"}
	cmd.AddCommand(newActivityStartCmd(), newActivityUpdateCmd(), newActivityEndCmd(), newActivityGetCmd())
	return cmd
}

func newActivityStartCmd() *cobra.Command {
	var req beam.ActivityRequest
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a Live Activity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			data, err := postJSON(client, hookURL(cfg, "/live-activities"), req, "")
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		},
	}
	activityFlags(cmd, &req)
	cmd.Flags().StringVar(&req.Key, "key", "", "stable activity key")
	cmd.Flags().BoolVar(&req.Replace, "replace", false, "replace an existing activity for the key or device")
	return cmd
}

func newActivityUpdateCmd() *cobra.Command {
	var req beam.ActivityRequest
	cmd := &cobra.Command{
		Use:   "update <id-or-key>",
		Short: "Update a Live Activity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			data, err := patchJSON(client, hookURL(cfg, "/live-activities/"+args[0]), req)
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		},
	}
	activityFlags(cmd, &req)
	return cmd
}

func newActivityEndCmd() *cobra.Command {
	var req beam.ActivityRequest
	cmd := &cobra.Command{
		Use:   "end <id-or-key>",
		Short: "End a Live Activity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			data, err := postJSON(client, hookURL(cfg, "/live-activities/"+args[0]+"/end"), req, "")
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		},
	}
	activityFlags(cmd, &req)
	return cmd
}

func newActivityGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id-or-key>",
		Short: "Read a Live Activity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			data, err := getJSON(client, hookURL(cfg, "/live-activities/"+args[0]))
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		},
	}
}

func activityFlags(cmd *cobra.Command, req *beam.ActivityRequest) {
	cmd.Flags().StringVar(&req.Title, "title", "", "activity title")
	cmd.Flags().StringVar(&req.Status, "status", "", "activity status")
	cmd.Flags().StringVar(&req.Symbol, "symbol", "", "symbol name")
	cmd.Flags().StringVar(&req.AccentColor, "accent-color", "", "accent color as #RRGGBB")
	cmd.Flags().StringVar(&req.Style, "style", "", "layout style")
	cmd.Flags().StringVar(&req.PrivacyMode, "privacy-mode", "", "privacy mode")
	cmd.Flags().Float64("progress", -1, "progress from 0 to 1")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		progress, err := cmd.Flags().GetFloat64("progress")
		if err != nil {
			return err
		}
		if progress >= 0 {
			req.Progress = &progress
		}
		return nil
	}
}
