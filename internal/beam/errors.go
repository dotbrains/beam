package beam

import "errors"

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
