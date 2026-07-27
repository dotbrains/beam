package beam

import (
	"fmt"
	"time"
)

const (
	defaultRequestsPerMinute = 600
	defaultMonthlyOperations = 100000
)

type LimitError struct {
	Kind       string        `json:"kind"`
	Limit      int           `json:"limit"`
	RetryAfter time.Duration `json:"-"`
	ResetAt    time.Time     `json:"resetAt"`
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
	if service.Limits.RequestsPerMinute == 0 {
		service.Limits.RequestsPerMinute = defaultRequestsPerMinute
	}
	if service.Limits.MonthlyOperations == 0 {
		service.Limits.MonthlyOperations = defaultMonthlyOperations
	}
	if !service.Limits.PermissiveDefaults && service.Limits.RequestsPerMinute == defaultRequestsPerMinute &&
		service.Limits.MonthlyOperations == defaultMonthlyOperations {
		service.Limits.PermissiveDefaults = true
	}
}

func consumeServiceOperation(service *Service, now time.Time) (LimitError, bool) {
	normalizeServiceLimits(service)
	resetUsageWindows(&service.Usage, now)
	if service.Limits.RequestsPerMinute > 0 && service.Usage.MinuteOperations >= service.Limits.RequestsPerMinute {
		resetAt := service.Usage.MinuteWindowStartedAt.Add(time.Minute)
		return LimitError{
			Kind:       "rate_limit",
			Limit:      service.Limits.RequestsPerMinute,
			RetryAfter: resetAt.Sub(now).Round(time.Second),
			ResetAt:    resetAt,
		}, true
	}
	if service.Limits.MonthlyOperations > 0 && service.Usage.MonthOperations >= service.Limits.MonthlyOperations {
		resetAt := nextMonthStart(now)
		return LimitError{
			Kind:       "monthly_allowance",
			Limit:      service.Limits.MonthlyOperations,
			RetryAfter: resetAt.Sub(now).Round(time.Second),
			ResetAt:    resetAt,
		}, true
	}
	service.Usage.MinuteOperations++
	service.Usage.MonthOperations++
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
