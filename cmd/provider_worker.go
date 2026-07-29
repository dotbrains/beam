package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dotbrains/beam/internal/beam"
	"github.com/spf13/cobra"
)

type workerDeliveryRequest struct {
	Operation string `json:"operation"`
	EventID   string `json:"eventId,omitempty"`
	Activity  *struct {
		State    beam.ActivityState `json:"state"`
		Sequence int                `json:"sequence"`
	} `json:"activity,omitempty"`
	DeviceIDs []string             `json:"deviceIds,omitempty"`
	Devices   []workerTargetDevice `json:"devices,omitempty"`
	CreatedAt time.Time            `json:"createdAt"`
}

type workerTargetDevice struct {
	DeviceID         string `json:"deviceId"`
	PushToken        string `json:"pushToken,omitempty"`
	PushToStartToken string `json:"pushToStartToken,omitempty"`
}

func newProviderWorkerCmd() *cobra.Command {
	var addr, token, providerName string
	cmd := &cobra.Command{
		Use:   "provider-worker",
		Short: "Run a token-safe HTTP push provider worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			server := &http.Server{Addr: addr, Handler: providerWorkerHandler(providerName, token)}
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "beam provider worker listening on %s\n", addr); err != nil {
				return err
			}
			return server.ListenAndServe()
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8081", "listen address")
	cmd.Flags().StringVar(&token, "token", "", "required bearer token")
	cmd.Flags().StringVar(&providerName, "provider-name", "http-worker", "provider name for diagnostics")
	return cmd
}

func providerWorkerHandler(providerName, token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeWorkerJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method_not_allowed"})
			return
		}
		writeWorkerJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/deliver", func(w http.ResponseWriter, r *http.Request) {
		handleProviderDelivery(w, r, providerName, token)
	})
	return mux
}

func handleProviderDelivery(w http.ResponseWriter, r *http.Request, providerName, token string) {
	if r.Method != http.MethodPost {
		writeWorkerJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method_not_allowed"})
		return
	}
	if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
		writeWorkerJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}
	var req workerDeliveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWorkerJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	diagnostics := workerDiagnostics(providerName, req)
	writeWorkerJSON(w, http.StatusOK, map[string]any{"diagnostics": diagnostics})
}

func workerDiagnostics(providerName string, req workerDeliveryRequest) []beam.ProviderDiagnostic {
	now := req.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if providerName == "" {
		providerName = "http-worker"
	}
	targets := req.Devices
	if len(targets) == 0 {
		for _, id := range req.DeviceIDs {
			targets = append(targets, workerTargetDevice{DeviceID: id})
		}
	}
	if len(targets) == 0 {
		return []beam.ProviderDiagnostic{{Provider: providerName, Operation: req.Operation, Status: "skipped", Reason: "no_active_device", CreatedAt: now}}
	}
	diagnostics := make([]beam.ProviderDiagnostic, 0, len(targets))
	for _, target := range targets {
		diagnostics = append(diagnostics, workerDiagnostic(providerName, req.Operation, target, now))
	}
	return diagnostics
}

func workerDiagnostic(providerName, operation string, target workerTargetDevice, now time.Time) beam.ProviderDiagnostic {
	status, reason := "accepted", ""
	if !workerHasToken(operation, target) {
		status, reason = "skipped", "missing_push_token"
	}
	return beam.ProviderDiagnostic{Provider: providerName, Operation: operation, DeviceID: target.DeviceID, Status: status, Reason: reason, CreatedAt: now}
}

func workerHasToken(operation string, target workerTargetDevice) bool {
	if strings.TrimSpace(target.PushToken) != "" {
		return true
	}
	return operation == "activity_start" && strings.TrimSpace(target.PushToStartToken) != ""
}

func writeWorkerJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
