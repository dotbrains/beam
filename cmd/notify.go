package cmd

import (
	"encoding/json"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/spf13/cobra"
)

func newNotifyCmd() *cobra.Command {
	var title, imageURL, url, idempotencyKey string
	cmd := &cobra.Command{
		Use:   "notify <body>",
		Short: "Send a one-shot notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			payload := beam.NotificationRequest{
				Body:     args[0],
				Title:    title,
				ImageURL: imageURL,
				URL:      url,
			}
			data, err := postJSON(client, hookURL(cfg, ""), payload, idempotencyKey)
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
	cmd.Flags().StringVar(&title, "title", "", "sender title")
	cmd.Flags().StringVar(&imageURL, "image", "", "sender image URL")
	cmd.Flags().StringVar(&url, "url", "", "tap destination URL")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "safe retry key")
	return cmd
}
