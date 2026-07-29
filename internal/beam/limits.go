package beam

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

const (
	defaultRequestsPerMinute = 600
	defaultMonthlyOperations = 100000
	idempotencyRetention     = 24 * time.Hour
)

type LimitError struct {
	Kind       string        `json:"kind"`
	Limit      int           `json:"limit"`
	RetryAfter time.Duration `json:"-"`
	ResetAt    time.Time     `json:"resetAt"`
}

type IdempotencyRecord struct {
	Fingerprint string
	EventID     string
	ActivityID  string
	CreatedAt   time.Time
}

func (err LimitError) Error() string {
	if err.Kind == "monthly_allowance" {
		return ErrAllowanceExceeded.Error()
	}
	return ErrRateLimited.Error()
}

func (err LimitError) Unwrap() error {
	if err.Kind == "monthly_allowance" {
		return ErrAllowanceExceeded
	}
	return ErrRateLimited
}

func defaultServiceLimits() ServiceLimits {
	return ServiceLimits{
		RequestsPerMinute:  defaultRequestsPerMinute,
		MonthlyOperations:  defaultMonthlyOperations,
		DeviceRouting:      true,
		PermissiveDefaults: true,
	}
}

func normalizeServiceLimits(service *Service) {
	if service.Limits == (ServiceLimits{}) {
		service.Limits = defaultServiceLimits()
		return
	}
	if service.Limits.RequestsPerMinute == 0 {
		service.Limits.RequestsPerMinute = defaultRequestsPerMinute
	}
	if service.Limits.MonthlyOperations == 0 {
		service.Limits.MonthlyOperations = defaultMonthlyOperations
	}
	if service.Limits.PermissiveDefaults {
		service.Limits.DeviceRouting = true
	}
}

func normalizeServiceLimitsForAccount(limits *ServiceLimits) {
	if limits.RequestsPerMinute == 0 {
		limits.RequestsPerMinute = defaultRequestsPerMinute
	}
	if limits.MonthlyOperations == 0 {
		limits.MonthlyOperations = defaultMonthlyOperations
	}
}

func consumeOperation(service *Service, account *Account, now time.Time) (LimitError, bool) {
	normalizeServiceLimits(service)
	if account != nil {
		normalizeServiceLimitsForAccount(&account.Limits)
		resetUsageWindows(&account.Usage, now)
	}
	resetUsageWindows(&service.Usage, now)
	if limit, limited := operationLimit(service.Limits, service.Usage, now); limited {
		return limit, true
	}
	if account != nil {
		if limit, limited := operationLimit(account.Limits, account.Usage, now); limited {
			return limit, true
		}
		account.Usage.MinuteOperations++
		account.Usage.MonthOperations++
	}
	service.Usage.MinuteOperations++
	service.Usage.MonthOperations++
	return LimitError{}, false
}

func operationLimit(limits ServiceLimits, usage ServiceUsage, now time.Time) (LimitError, bool) {
	if limits.RequestsPerMinute > 0 && usage.MinuteOperations >= limits.RequestsPerMinute {
		resetAt := usage.MinuteWindowStartedAt.Add(time.Minute)
		return LimitError{Kind: "rate_limit", Limit: limits.RequestsPerMinute, RetryAfter: retryAfterUntil(resetAt, now), ResetAt: resetAt}, true
	}
	if limits.MonthlyOperations > 0 && usage.MonthOperations >= limits.MonthlyOperations {
		resetAt := nextMonthStart(now)
		return LimitError{Kind: "monthly_allowance", Limit: limits.MonthlyOperations, RetryAfter: retryAfterUntil(resetAt, now), ResetAt: resetAt}, true
	}
	return LimitError{}, false
}

func resetUsageWindows(usage *ServiceUsage, now time.Time) {
	if usage.MinuteWindowStartedAt.IsZero() || now.Sub(usage.MinuteWindowStartedAt) >= time.Minute {
		usage.MinuteWindowStartedAt = now.Truncate(time.Minute)
		usage.MinuteOperations = 0
	}
	window := monthWindow(now)
	if usage.MonthWindow != window {
		usage.MonthWindow = window
		usage.MonthOperations = 0
	}
}

func monthWindow(t time.Time) string {
	return fmt.Sprintf("%04d-%02d", t.UTC().Year(), t.UTC().Month())
}

func nextMonthStart(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

func retryAfterUntil(resetAt, now time.Time) time.Duration {
	retryAfter := resetAt.Sub(now).Round(time.Second)
	if retryAfter <= 0 && resetAt.After(now) {
		return time.Second
	}
	return retryAfter
}

func optionalInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func optionalDurationOrDefault(value *int, fallbackSeconds int) time.Duration {
	if value == nil {
		return time.Duration(fallbackSeconds) * time.Second
	}
	return time.Duration(*value) * time.Second
}

func durationOrDefault(seconds, fallback int) time.Duration {
	if seconds == 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func randomID() string {
	var buf [9]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf[:]), "=")
}

func pruneIdempotencyRecords(records map[string]IdempotencyRecord, now time.Time) {
	for key, record := range records {
		if record.CreatedAt.IsZero() {
			continue
		}
		if now.Sub(record.CreatedAt) > idempotencyRetention {
			delete(records, key)
		}
	}
}

func activityTargetDeviceIDs(devices []Device, requestedIDs []string) []string {
	if len(requestedIDs) > 0 {
		return append([]string(nil), requestedIDs...)
	}
	targets := make([]string, 0, len(devices))
	for _, device := range devices {
		if device.Active {
			targets = append(targets, device.ID)
		}
	}
	return targets
}

func replacementKey(requested string, replaced map[string]Activity) string {
	if requested != "" {
		return requested
	}
	for _, activity := range replaced {
		if activity.Key != "" {
			return activity.Key
		}
	}
	return ""
}

func mergeActivity(activity *Activity, req ActivityRequest) {
	if req.Title != "" {
		activity.State.Title = req.Title
	}
	if req.Status != "" {
		activity.State.Status = req.Status
	}
	if req.DetailSet || req.Detail != nil {
		activity.State.Detail = req.Detail
	}
	if req.ProgressSet || req.Progress != nil {
		activity.State.Progress = req.Progress
	}
	if req.Symbol != "" {
		activity.State.Symbol = req.Symbol
	}
	if req.AccentColor != "" {
		activity.State.AccentColor = req.AccentColor
	}
	if req.Style != "" {
		activity.State.Style = req.Style
	}
	if req.PrivacyMode != "" {
		activity.State.PrivacyMode = req.PrivacyMode
	}
}

func isAppliedActivityRetry(activity Activity, req ActivityRequest) bool {
	if req.IfSequence == nil || *req.IfSequence != activity.Sequence-1 {
		return false
	}
	if !hasComparableActivityStateField(req) {
		return false
	}
	return activityMatchesRequest(activity, req)
}

func hasComparableActivityStateField(req ActivityRequest) bool {
	return req.Title != "" || req.Status != "" || req.DetailSet || req.Detail != nil || req.ProgressSet || req.Progress != nil ||
		req.Symbol != "" || req.AccentColor != "" || req.Style != "" || req.PrivacyMode != ""
}

func activityMatchesRequest(activity Activity, req ActivityRequest) bool {
	if req.Title != "" && activity.State.Title != req.Title {
		return false
	}
	if req.Status != "" && activity.State.Status != req.Status {
		return false
	}
	if (req.DetailSet || req.Detail != nil) && !stringPointersEqual(activity.State.Detail, req.Detail) {
		return false
	}
	if (req.ProgressSet || req.Progress != nil) && !floatPointersEqual(activity.State.Progress, req.Progress) {
		return false
	}
	if req.Symbol != "" && activity.State.Symbol != req.Symbol {
		return false
	}
	if req.AccentColor != "" && activity.State.AccentColor != req.AccentColor {
		return false
	}
	if req.Style != "" && activity.State.Style != req.Style {
		return false
	}
	if req.PrivacyMode != "" && activity.State.PrivacyMode != req.PrivacyMode {
		return false
	}
	return true
}

func stringPointersEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func floatPointersEqual(left, right *float64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func activityStateFromStart(req ActivityRequest) ActivityState {
	return ActivityState{
		Title:       req.Title,
		Status:      req.Status,
		Detail:      req.Detail,
		Progress:    req.Progress,
		Symbol:      firstNonEmpty(req.Symbol, "terminal"),
		AccentColor: firstNonEmpty(req.AccentColor, "#5ED8B7"),
		Style:       firstNonEmpty(req.Style, "standard"),
		PrivacyMode: firstNonEmpty(req.PrivacyMode, "standard"),
	}
}

func (s *Store) activityForService(serviceID, id string) (Activity, bool) {
	activity, ok := s.activities[id]
	if !ok {
		activity, ok = s.activities[activityKey(serviceID, id)]
	}
	return activity, ok && activity.ServiceID == serviceID
}

func (s *Store) storeExpiredActivity(serviceID string, activity Activity) {
	activity.Status = "expired"
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activityKey(serviceID, activity.Key)] = activity
	}
}

func activityKey(serviceID, key string) string {
	return serviceID + ":" + key
}

func overlaps(first, second []string) bool {
	seen := map[string]bool{}
	for _, value := range first {
		seen[value] = true
	}
	for _, value := range second {
		if seen[value] {
			return true
		}
	}
	return false
}
