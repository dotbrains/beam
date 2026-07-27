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
	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		handleServices(store, w, r)
	})
	mux.HandleFunc("/api/auth/device", func(w http.ResponseWriter, r *http.Request) {
		handleAuthDevice(store, w, r)
	})
	mux.HandleFunc("/api/auth/device/", func(w http.ResponseWriter, r *http.Request) {
		handleAuthDevicePath(store, w, r)
	})
	mux.HandleFunc("/api/services/", func(w http.ResponseWriter, r *http.Request) {
		handleService(store, w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	return mux
}

func handleAuthDevice(store Backend, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
		return
	}
	body := readBody(w, r)
	if body == nil {
		return
	}
	var req AuthDeviceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
		return
	}
	device, err := store.StartAuthDevice(req, publicBaseURL(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "device": device})
}

func handleAuthDevicePath(store Backend, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/auth/device/"), "/")
	if len(parts) == 2 && parts[1] == "token" && r.Method == http.MethodGet {
		device, err := store.AuthDeviceToken(parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, authDeviceTokenResponse(device))
		return
	}
	if len(parts) == 2 && parts[1] == "approve" && r.Method == http.MethodPost {
		device, err := store.ApproveAuthDevice(parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, authDeviceTokenResponse(device))
		return
	}
	writeJSON(w, http.StatusNotFound, errorBody("Not found"))
}

func handleServices(store Backend, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "services": store.Services()})
	case http.MethodPost:
		body := readBody(w, r)
		if body == nil {
			return
		}
		var req ServiceCreateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
			return
		}
		resp, err := store.CreateService(req)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "service": resp.Service, "token": resp.Token})
	default:
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
	}
}

func handleService(store Backend, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
		return
	}
	id := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		service, err := store.Service(id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": service})
	case len(parts) == 1 && r.Method == http.MethodPatch:
		req, ok := decodeServiceUpdate(w, r)
		if !ok {
			return
		}
		service, err := store.UpdateService(id, req)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": service})
	case len(parts) == 1 && r.Method == http.MethodDelete:
		if err := store.DeleteService(id); err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 2 && parts[1] == "rotate-token" && r.Method == http.MethodPost:
		resp, err := store.RotateServiceToken(id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": resp.Service, "token": resp.Token})
	case len(parts) == 2 && parts[1] == "devices" && r.Method == http.MethodGet:
		devices, err := store.Devices(id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "devices": devices})
	case len(parts) == 2 && parts[1] == "devices" && r.Method == http.MethodPost:
		body := readBody(w, r)
		if body == nil {
			return
		}
		var req DeviceRegisterRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
			return
		}
		device, err := store.RegisterDevice(id, req)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "device": device})
	case len(parts) == 4 && parts[1] == "devices" && parts[3] == "deactivate" && r.Method == http.MethodPost:
		device, err := store.DeactivateDevice(id, parts[2])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "device": device})
	default:
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
	}
}

func decodeServiceUpdate(w http.ResponseWriter, r *http.Request) (ServiceUpdateRequest, bool) {
	body := readBody(w, r)
	if body == nil {
		return ServiceUpdateRequest{}, false
	}
	var req ServiceUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
		return ServiceUpdateRequest{}, false
	}
	return req, true
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
	case len(parts) == 2 && parts[1] == "live-activities" && r.Method == http.MethodGet:
		activities, err := store.Activities(token)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		items := make([]map[string]any, 0, len(activities))
		for _, activity := range activities {
			items = append(items, activityResponse(activity))
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "activities": items})
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
		"key":        activity.Key,
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

func authDeviceTokenResponse(device AuthDevice) map[string]any {
	resp := map[string]any{
		"ok":     true,
		"status": device.Status,
	}
	if device.Status == "approved" {
		resp["token"] = device.Token
		resp["scopes"] = device.Scopes
		resp["clientName"] = device.ClientName
		resp["expiresAt"] = device.ExpiresAt
	}
	return resp
}

func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	return scheme + "://" + host
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
