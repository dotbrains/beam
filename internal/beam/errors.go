package beam

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrUnknownWebhook      = errors.New("unknown webhook")
	ErrInvalidPayload      = errors.New("invalid payload")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different payload")
	ErrPendingRequest      = errors.New("idempotent request is still processing")
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("conflict")
	ErrTerminalActivity    = errors.New("live activity is already terminal")
	ErrSequenceConflict    = errors.New("sequence conflict")
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrAllowanceExceeded   = errors.New("monthly allowance exceeded")
	ErrPaymentRequired     = errors.New("payment required")
	ErrProviderFailure     = errors.New("push provider failure")
	ErrUnauthorized        = errors.New("authentication required")
	ErrForbidden           = errors.New("insufficient scope")
)

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": err.Error(), "code": "unauthorized"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": err.Error(), "code": "forbidden"})
	case errors.Is(err, ErrUnknownWebhook):
		writeJSON(w, http.StatusNotFound, errorBody("Unknown webhook"))
	case errors.Is(err, ErrValidation):
		body := errorBody("Invalid payload")
		body["fields"] = validationFields(err)
		writeJSON(w, http.StatusBadRequest, body)
	case errors.Is(err, ErrInvalidPayload):
		writeJSON(w, http.StatusBadRequest, errorBody("Invalid payload"))
	case errors.Is(err, ErrIdempotencyConflict), errors.Is(err, ErrConflict), errors.Is(err, ErrSequenceConflict), errors.Is(err, ErrTerminalActivity):
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error(), "code": conflictCode(err)})
	case errors.Is(err, ErrRateLimited), errors.Is(err, ErrAllowanceExceeded):
		writeLimitError(w, err)
	case errors.Is(err, ErrPaymentRequired):
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"ok": false, "error": err.Error(), "code": "payment_required"})
	case errors.Is(err, ErrProviderFailure):
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error(), "code": "provider_failure"})
	case errors.Is(err, ErrPendingRequest):
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
	}
}

func conflictCode(err error) string {
	switch {
	case errors.Is(err, ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, ErrSequenceConflict):
		return "sequence_conflict"
	case errors.Is(err, ErrTerminalActivity):
		return "terminal_activity"
	default:
		return "conflict"
	}
}

func writeLimitError(w http.ResponseWriter, err error) {
	var limitErr LimitError
	if !errors.As(err, &limitErr) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": ErrRateLimited.Error(), "code": "rate_limit"})
		return
	}
	if limitErr.RetryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", limitErr.RetryAfter.Seconds()))
	}
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"ok":         false,
		"error":      err.Error(),
		"code":       limitErr.Kind,
		"limit":      limitErr.Limit,
		"retryAfter": int(limitErr.RetryAfter.Seconds()),
		"resetAt":    limitErr.ResetAt,
	})
}

func errorBody(message string) map[string]any {
	return map[string]any{"ok": false, "error": message, "code": errorCode(message)}
}

func errorCode(message string) string {
	switch strings.ToLower(strings.TrimSpace(message)) {
	case "unknown webhook":
		return "unknown_webhook"
	case "not found":
		return "not_found"
	case "method not allowed":
		return "method_not_allowed"
	case "invalid json":
		return "invalid_json"
	case "invalid payload":
		return "invalid_payload"
	case "unable to read request":
		return "request_read_failed"
	default:
		return "error"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
