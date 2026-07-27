package beam

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func Handler(store Backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/hooks/", func(w http.ResponseWriter, r *http.Request) {
		handleHook(store, w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	return mux
}

func handleHook(store Backend, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/hooks/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, errorBody("Unknown webhook"))
		return
	}
	token := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodPost:
		body := readBody(w, r)
		if body == nil {
			return
		}
		var req NotificationRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
			return
		}
		if err := validateIdempotencyKey(r.Header.Get("Idempotency-Key")); err != nil {
			writeStoreError(w, err)
			return
		}
		event, idempotent, err := store.SendNotification(token, req, r.Header.Get("Idempotency-Key"), string(body))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		resp := map[string]any{"ok": true, "eventId": event.ID, "delivered": event.Delivered}
		if idempotent {
			resp["idempotent"] = true
		}
		if event.Response != nil {
			resp["response"] = event.Response
		}
		writeJSON(w, http.StatusOK, resp)
	case len(parts) == 3 && parts[1] == "events" && r.Method == http.MethodGet:
		event, err := store.Event(token, parts[2])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event": event})
	case len(parts) == 4 && parts[1] == "events" && parts[3] == "cancel" && r.Method == http.MethodPost:
		event, err := store.CancelEvent(token, parts[2])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event": event})
	case len(parts) == 2 && parts[1] == "live-activities" && r.Method == http.MethodPost:
		body := readBody(w, r)
		if body == nil {
			return
		}
		var req ActivityRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
			return
		}
		if err := validateIdempotencyKey(r.Header.Get("Idempotency-Key")); err != nil {
			writeStoreError(w, err)
			return
		}
		activity, idempotent, err := store.StartActivity(token, req, r.Header.Get("Idempotency-Key"), string(body))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		resp := activityResponse(activity)
		if idempotent {
			resp["idempotent"] = true
		}
		writeJSON(w, http.StatusCreated, resp)
	case len(parts) == 3 && parts[1] == "live-activities" && r.Method == http.MethodGet:
		activity, err := store.Activity(token, parts[2])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, activityResponse(activity))
	case len(parts) == 3 && parts[1] == "live-activities" && r.Method == http.MethodPatch:
		req, ok := decodeActivity(w, r)
		if !ok {
			return
		}
		activity, err := store.UpdateActivity(token, parts[2], req)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, activityResponse(activity))
	case len(parts) == 4 && parts[1] == "live-activities" && parts[3] == "end" && r.Method == http.MethodPost:
		req, ok := decodeActivity(w, r)
		if !ok {
			return
		}
		activity, err := store.EndActivity(token, parts[2], req)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, activityResponse(activity))
	default:
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
	}
}

func decodeActivity(w http.ResponseWriter, r *http.Request) (ActivityRequest, bool) {
	body := readBody(w, r)
	if body == nil {
		return ActivityRequest{}, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return ActivityRequest{}, true
	}
	var req ActivityRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
		return ActivityRequest{}, false
	}
	return req, true
}

func readBody(w http.ResponseWriter, r *http.Request) []byte {
	defer r.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Unable to read request"})
		return nil
	}
	return buf.Bytes()
}

func activityResponse(activity Activity) map[string]any {
	return map[string]any{
		"ok":         true,
		"activityId": activity.ID,
		"sequence":   activity.Sequence,
		"status":     activity.Status,
		"accepted":   1,
		"failed":     0,
		"state":      activity.State,
		"expiresAt":  activity.ExpiresAt,
		"staleAt":    activity.StaleAt,
		"endedAt":    activity.EndedAt,
	}
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnknownWebhook):
		writeJSON(w, http.StatusNotFound, errorBody("Unknown webhook"))
	case errors.Is(err, ErrValidation):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid payload", "fields": validationFields(err)})
	case errors.Is(err, ErrInvalidPayload):
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid payload"})
	case errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrSequenceConflict), errors.Is(err, ErrTerminalActivity):
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
	case errors.Is(err, ErrPendingRequest):
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
	}
}

func errorBody(message string) map[string]any {
	return map[string]any{"ok": false, "error": message}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
