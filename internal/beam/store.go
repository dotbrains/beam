package beam

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
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
)

type Store struct {
	mu          sync.Mutex
	services    map[string]Service
	events      map[string]Event
	activities  map[string]Activity
	authDevices map[string]AuthDevice
	idempotency map[string]IdempotencyRecord
}

type NotificationRequest struct {
	Body      string           `json:"body"`
	Title     string           `json:"title,omitempty"`
	ImageURL  string           `json:"imageUrl,omitempty"`
	URL       string           `json:"url,omitempty"`
	DeviceIDs []string         `json:"deviceIds,omitempty"`
	Response  *ResponseRequest `json:"response,omitempty"`
}

type Event struct {
	ID        string         `json:"id"`
	ServiceID string         `json:"serviceId,omitempty"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	ImageURL  string         `json:"imageUrl,omitempty"`
	URL       string         `json:"url,omitempty"`
	Delivered int            `json:"delivered"`
	Response  *ResponseState `json:"response,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type ActivityRequest struct {
	Title               string   `json:"title,omitempty"`
	Status              string   `json:"status,omitempty"`
	Detail              *string  `json:"detail,omitempty"`
	Progress            *float64 `json:"progress,omitempty"`
	Symbol              string   `json:"symbol,omitempty"`
	AccentColor         string   `json:"accentColor,omitempty"`
	Style               string   `json:"style,omitempty"`
	PrivacyMode         string   `json:"privacyMode,omitempty"`
	Key                 string   `json:"key,omitempty"`
	DeviceIDs           []string `json:"deviceIds,omitempty"`
	Replace             bool     `json:"replace,omitempty"`
	IfSequence          *int     `json:"ifSequence,omitempty"`
	ExpiresInSeconds    int      `json:"expiresInSeconds,omitempty"`
	StaleAfterSeconds   int      `json:"staleAfterSeconds,omitempty"`
	DismissAfterSeconds int      `json:"dismissAfterSeconds,omitempty"`
}

type Activity struct {
	ID        string        `json:"id"`
	Key       string        `json:"key,omitempty"`
	DeviceIDs []string      `json:"deviceIds,omitempty"`
	Sequence  int           `json:"sequence"`
	Status    string        `json:"status"`
	State     ActivityState `json:"state"`
	Delivered int           `json:"delivered"`
	ExpiresAt time.Time     `json:"expiresAt"`
	StaleAt   time.Time     `json:"staleAt"`
	EndedAt   *time.Time    `json:"endedAt"`
	CreatedAt time.Time     `json:"createdAt"`
}

type ActivityState struct {
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Detail      *string  `json:"detail,omitempty"`
	Progress    *float64 `json:"progress,omitempty"`
	Symbol      string   `json:"symbol"`
	AccentColor string   `json:"accentColor"`
	Style       string   `json:"style"`
	PrivacyMode string   `json:"privacyMode"`
}

type IdempotencyRecord struct {
	Fingerprint string
	EventID     string
	ActivityID  string
	CreatedAt   time.Time
}

func NewStore() *Store {
	store := &Store{
		services:    map[string]Service{},
		events:      map[string]Event{},
		activities:  map[string]Activity{},
		authDevices: map[string]AuthDevice{},
		idempotency: map[string]IdempotencyRecord{},
	}
	now := time.Now().UTC()
	store.RegisterService(Service{
		ID:        "svc_dev",
		Token:     "dev_token",
		Title:     "Beam",
		Devices:   []Device{{ID: "dev_local", Name: "Local Device", Platform: "ios", Active: true, CreatedAt: now, UpdatedAt: now}},
		CreatedAt: now,
		UpdatedAt: now,
	})
	return store
}

func (s *Store) SendNotification(token string, req NotificationRequest, idemKey, fingerprint string) (Event, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Event{}, false, ErrUnknownWebhook
	}
	if err := validateNotification(req); err != nil {
		return Event{}, false, err
	}
	if len(req.DeviceIDs) > 0 && !service.Limits.DeviceRouting {
		return Event{}, false, ErrPaymentRequired
	}
	if err := validateDeviceRouting(service.Devices, req.DeviceIDs); err != nil {
		return Event{}, false, err
	}
	pruneIdempotencyRecords(s.idempotency, time.Now().UTC())
	if idemKey != "" {
		recordKey := token + ":" + idemKey
		if record, ok := s.idempotency[recordKey]; ok {
			if record.Fingerprint != fingerprint {
				return Event{}, false, ErrIdempotencyConflict
			}
			event, ok := s.events[record.EventID]
			if !ok {
				return Event{}, false, ErrPendingRequest
			}
			return event, true, nil
		}
	}
	limit, limited := consumeServiceOperation(&service, time.Now().UTC())
	if limited {
		return Event{}, false, limit
	}
	s.services[token] = service
	title := firstNonEmpty(req.Title, service.Title, "Beam")
	event := Event{
		ID:        "evt_" + randomID(),
		ServiceID: service.ID,
		Title:     title,
		Body:      strings.TrimSpace(req.Body),
		ImageURL:  firstNonEmpty(req.ImageURL, service.ImageURL),
		URL:       firstNonEmpty(req.URL, service.URL),
		Delivered: deliveredDeviceCount(service.Devices, req.DeviceIDs),
		CreatedAt: time.Now().UTC(),
	}
	if req.Response != nil {
		expires := req.Response.ExpiresInSeconds
		if expires == 0 {
			expires = 900
		}
		event.Response = &ResponseState{
			Type:          req.Response.Type,
			Status:        "pending",
			CorrelationID: req.Response.CorrelationID,
			ExpiresAt:     time.Now().UTC().Add(time.Duration(expires) * time.Second),
		}
		if req.Response.Callback != nil {
			event.Response.CallbackURL = req.Response.Callback.URL
			event.Response.CallbackToken = req.Response.Callback.Token
		}
	}
	s.events[event.ID] = event
	if idemKey != "" {
		s.idempotency[token+":"+idemKey] = IdempotencyRecord{Fingerprint: fingerprint, EventID: event.ID, CreatedAt: time.Now().UTC()}
	}
	return event, false, nil
}

func (s *Store) Event(token, id string) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Event{}, ErrUnknownWebhook
	}
	event, ok := s.events[id]
	if !ok || !eventVisibleToService(event, service) {
		return Event{}, ErrNotFound
	}
	if event.Response != nil && event.Response.Status == "pending" && time.Now().UTC().After(event.Response.ExpiresAt) {
		event.Response.Status = "expired"
		s.events[id] = event
	}
	return event, nil
}

func (s *Store) CancelEvent(token, id string) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Event{}, ErrUnknownWebhook
	}
	event, ok := s.events[id]
	if !ok || !eventVisibleToService(event, service) || event.Response == nil || event.Response.Status != "pending" {
		return Event{}, ErrNotFound
	}
	event.Response.Status = "canceled"
	now := time.Now().UTC()
	event.Response.RespondedAt = &now
	s.events[id] = event
	return event, nil
}

func (s *Store) StartActivity(token string, req ActivityRequest, idemKey, fingerprint string) (Activity, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Activity{}, false, ErrUnknownWebhook
	}
	if err := validateActivityStart(req); err != nil {
		return Activity{}, false, err
	}
	if len(req.DeviceIDs) > 0 && !service.Limits.DeviceRouting {
		return Activity{}, false, ErrPaymentRequired
	}
	if err := validateDeviceRouting(service.Devices, req.DeviceIDs); err != nil {
		return Activity{}, false, err
	}
	pruneIdempotencyRecords(s.idempotency, time.Now().UTC())
	if idemKey != "" {
		recordKey := token + ":activity:" + idemKey
		if record, ok := s.idempotency[recordKey]; ok {
			if record.Fingerprint != fingerprint {
				return Activity{}, false, ErrIdempotencyConflict
			}
			return s.activities[record.ActivityID], true, nil
		}
	}
	targets := activityTargetDeviceIDs(service.Devices, req.DeviceIDs)
	replaced := map[string]Activity{}
	for _, existing := range s.activities {
		if existing.EndedAt != nil || time.Now().UTC().After(existing.ExpiresAt) {
			continue
		}
		if existing.ID != "" && (existing.Key == req.Key && req.Key != "" || overlaps(existing.DeviceIDs, targets)) {
			if !req.Replace {
				return Activity{}, false, ErrConflict
			}
			replaced[existing.ID] = existing
		}
	}
	limit, limited := consumeServiceOperation(&service, time.Now().UTC())
	if limited {
		return Activity{}, false, limit
	}
	s.services[token] = service
	now := time.Now().UTC()
	for _, replaced := range replaced {
		replaced.Status = "ended"
		replaced.Sequence++
		replaced.EndedAt = &now
		s.activities[replaced.ID] = replaced
		delete(s.activities, replaced.Key)
	}
	expires := durationOrDefault(req.ExpiresInSeconds, 28800)
	stale := durationOrDefault(req.StaleAfterSeconds, 14400)
	activity := Activity{
		ID:        "act_" + randomID(),
		Key:       req.Key,
		DeviceIDs: targets,
		Sequence:  0,
		Status:    "active",
		Delivered: len(targets),
		CreatedAt: now,
		ExpiresAt: now.Add(expires),
		StaleAt:   now.Add(stale),
		State: ActivityState{
			Title:       req.Title,
			Status:      req.Status,
			Detail:      req.Detail,
			Progress:    req.Progress,
			Symbol:      firstNonEmpty(req.Symbol, "terminal"),
			AccentColor: firstNonEmpty(req.AccentColor, "#5ED8B7"),
			Style:       firstNonEmpty(req.Style, "standard"),
			PrivacyMode: firstNonEmpty(req.PrivacyMode, "standard"),
		},
	}
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activity.Key] = activity
	}
	if idemKey != "" {
		s.idempotency[token+":activity:"+idemKey] = IdempotencyRecord{Fingerprint: fingerprint, ActivityID: activity.ID, CreatedAt: now}
	}
	return activity, false, nil
}

func (s *Store) UpdateActivity(token, id string, req ActivityRequest) (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activities[id]
	if !ok {
		return Activity{}, ErrNotFound
	}
	if activity.EndedAt != nil || time.Now().UTC().After(activity.ExpiresAt) {
		return Activity{}, ErrTerminalActivity
	}
	if err := validateActivityUpdate(req); err != nil {
		return Activity{}, err
	}
	if req.IfSequence != nil && *req.IfSequence != activity.Sequence {
		return activity, ErrSequenceConflict
	}
	limit, limited := consumeServiceOperation(&service, time.Now().UTC())
	if limited {
		return Activity{}, limit
	}
	s.services[token] = service
	mergeActivity(&activity, req)
	activity.Sequence++
	if req.StaleAfterSeconds != 0 {
		activity.StaleAt = time.Now().UTC().Add(time.Duration(req.StaleAfterSeconds) * time.Second)
	}
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activity.Key] = activity
	}
	return activity, nil
}

func (s *Store) Activities(token string) ([]Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[token]; !ok {
		return nil, ErrUnknownWebhook
	}
	seen := map[string]bool{}
	activities := make([]Activity, 0, len(s.activities))
	for _, activity := range s.activities {
		if seen[activity.ID] {
			continue
		}
		seen[activity.ID] = true
		if activity.EndedAt == nil && time.Now().UTC().After(activity.ExpiresAt) {
			activity.Status = "expired"
			s.activities[activity.ID] = activity
			if activity.Key != "" {
				s.activities[activity.Key] = activity
			}
		}
		activities = append(activities, activity)
	}
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].CreatedAt.Before(activities[j].CreatedAt)
	})
	return activities, nil
}

func (s *Store) Activity(token, id string) (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[token]; !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activities[id]
	if !ok {
		return Activity{}, ErrNotFound
	}
	if activity.EndedAt == nil && time.Now().UTC().After(activity.ExpiresAt) {
		activity.Status = "expired"
		s.activities[activity.ID] = activity
		if activity.Key != "" {
			s.activities[activity.Key] = activity
		}
	}
	return activity, nil
}

func (s *Store) EndActivity(token, id string, req ActivityRequest) (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[token]
	if !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activities[id]
	if !ok {
		return Activity{}, ErrNotFound
	}
	if activity.EndedAt != nil || time.Now().UTC().After(activity.ExpiresAt) {
		return Activity{}, ErrTerminalActivity
	}
	if err := validateActivityEnd(req); err != nil {
		return Activity{}, err
	}
	if req.IfSequence != nil && *req.IfSequence != activity.Sequence {
		return activity, ErrSequenceConflict
	}
	limit, limited := consumeServiceOperation(&service, time.Now().UTC())
	if limited {
		return Activity{}, limit
	}
	s.services[token] = service
	mergeActivity(&activity, req)
	activity.Sequence++
	now := time.Now().UTC()
	activity.Status = "ended"
	activity.EndedAt = &now
	if activity.State.Status == "" {
		activity.State.Status = "Complete"
	}
	if activity.State.Symbol == "" {
		activity.State.Symbol = "success"
	}
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activity.Key] = activity
	}
	return activity, nil
}

func mergeActivity(activity *Activity, req ActivityRequest) {
	if req.Title != "" {
		activity.State.Title = req.Title
	}
	if req.Status != "" {
		activity.State.Status = req.Status
	}
	if req.Detail != nil {
		activity.State.Detail = req.Detail
	}
	if req.Progress != nil {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func durationOrDefault(seconds, fallback int) time.Duration {
	if seconds == 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func randomID() string {
	var buf [9]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf[:]), "=")
}
