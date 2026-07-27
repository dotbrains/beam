package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/spf13/cobra"
)

func newAskCmd() *cobra.Command {
	var approval, yesNo, text, wait bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "ask <prompt>",
		Short: "Send an interactive prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := 0
			kind := ""
			for _, item := range []struct {
				on   bool
				kind string
			}{{approval, "approval"}, {yesNo, "yes_no"}, {text, "text"}} {
				if item.on {
					selected++
					kind = item.kind
				}
			}
			if selected != 1 {
				return fmt.Errorf("pass exactly one of --approval, --yes-no, or --text")
			}
			cfg, client, err := apiClient()
			if err != nil {
				return err
			}
			payload := beam.NotificationRequest{
				Body: args[0],
				Response: &beam.ResponseRequest{
					Type:             kind,
					ExpiresInSeconds: int(timeout.Seconds()),
				},
			}
			data, err := postJSON(client, hookURL(cfg, ""), payload, "")
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			if wait {
				eventID, _ := out["eventId"].(string)
				deadline := time.Now().Add(timeout)
				for eventID != "" && time.Now().Before(deadline) {
					data, err = getJSON(client, hookURL(cfg, "/events/"+eventID))
					if err != nil {
						return err
					}
					if err := json.Unmarshal(data, &out); err != nil {
						return err
					}
					raw, _ := json.Marshal(out["event"])
					var event beam.Event
					if err := json.Unmarshal(raw, &event); err == nil && event.Response != nil && event.Response.Status != "pending" {
						break
					}
					time.Sleep(2 * time.Second)
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		},
	}
	cmd.Flags().BoolVar(&approval, "approval", false, "ask for approve or deny")
	cmd.Flags().BoolVar(&yesNo, "yes-no", false, "ask for yes or no")
	cmd.Flags().BoolVar(&text, "text", false, "ask for a text reply")
	cmd.Flags().BoolVar(&wait, "wait", false, "poll until the prompt settles or timeout passes")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "prompt expiry and wait timeout")
	return cmd
}
