package beam

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu          sync.Mutex
	services    map[string]Service
	events      map[string]Event
	activities  map[string]Activity
	authDevices map[string]AuthDevice
	idempotency map[string]IdempotencyRecord
	provider    PushProvider
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
	ID                  string               `json:"id"`
	ServiceID           string               `json:"serviceId,omitempty"`
	Title               string               `json:"title"`
	Body                string               `json:"body"`
	ImageURL            string               `json:"imageUrl,omitempty"`
	URL                 string               `json:"url,omitempty"`
	Delivered           int                  `json:"delivered"`
	ProviderDiagnostics []ProviderDiagnostic `json:"providerDiagnostics,omitempty"`
	Response            *ResponseState       `json:"response,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
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
	ID                  string               `json:"id"`
	ServiceID           string               `json:"serviceId,omitempty"`
	Key                 string               `json:"key,omitempty"`
	DeviceIDs           []string             `json:"deviceIds,omitempty"`
	Sequence            int                  `json:"sequence"`
	Status              string               `json:"status"`
	State               ActivityState        `json:"state"`
	Delivered           int                  `json:"delivered"`
	ProviderDiagnostics []ProviderDiagnostic `json:"providerDiagnostics,omitempty"`
	ExpiresAt           time.Time            `json:"expiresAt"`
	StaleAt             time.Time            `json:"staleAt"`
	DismissAt           *time.Time           `json:"dismissAt,omitempty"`
	EndedAt             *time.Time           `json:"endedAt"`
	CreatedAt           time.Time            `json:"createdAt"`
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

func (s *Store) SendNotification(token string, req NotificationRequest, idemKey, fingerprint string) (Event, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceForToken(token)
	if !ok {
		return Event{}, false, ErrUnknownWebhook
	}
	tokenHash := hashToken(token)
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
		recordKey := tokenHash + ":" + idemKey
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
	s.services[service.TokenHash] = service
	title := firstNonEmpty(req.Title, service.Title, "Beam")
	now := time.Now().UTC()
	targets := activityTargetDeviceIDs(service.Devices, req.DeviceIDs)
	eventID := "evt_" + randomID()
	diagnostics, err := s.provider.SendNotification(PushNotification{EventID: eventID, DeviceIDs: targets, CreatedAt: now})
	if err != nil {
		return Event{}, false, err
	}
	event := Event{
		ID:                  eventID,
		ServiceID:           service.ID,
		Title:               title,
		Body:                strings.TrimSpace(req.Body),
		ImageURL:            firstNonEmpty(req.ImageURL, service.ImageURL),
		URL:                 firstNonEmpty(req.URL, service.URL),
		Delivered:           acceptedDeliveryCount(diagnostics),
		ProviderDiagnostics: diagnostics,
		CreatedAt:           now,
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
			ExpiresAt:     now.Add(time.Duration(expires) * time.Second),
		}
		if req.Response.Callback != nil {
			event.Response.CallbackURL = req.Response.Callback.URL
			event.Response.CallbackToken = req.Response.Callback.Token
		}
	}
	s.events[event.ID] = event
	if idemKey != "" {
		s.idempotency[tokenHash+":"+idemKey] = IdempotencyRecord{Fingerprint: fingerprint, EventID: event.ID, CreatedAt: time.Now().UTC()}
	}
	return event, false, nil
}

func (s *Store) Event(token, id string) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceForToken(token)
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
	service, ok := s.serviceForToken(token)
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
	service, ok := s.serviceForToken(token)
	if !ok {
		return Activity{}, false, ErrUnknownWebhook
	}
	tokenHash := hashToken(token)
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
		recordKey := tokenHash + ":activity:" + idemKey
		if record, ok := s.idempotency[recordKey]; ok {
			if record.Fingerprint != fingerprint {
				return Activity{}, false, ErrIdempotencyConflict
			}
			activity, ok := s.activities[record.ActivityID]
			if !ok {
				return Activity{}, false, ErrPendingRequest
			}
			return activity, true, nil
		}
	}
	targets := activityTargetDeviceIDs(service.Devices, req.DeviceIDs)
	replaced := map[string]Activity{}
	for _, existing := range s.activities {
		if existing.ServiceID != service.ID {
			continue
		}
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
	s.services[service.TokenHash] = service
	now := time.Now().UTC()
	for _, replaced := range replaced {
		replaced.Status = "ended"
		replaced.Sequence++
		replaced.EndedAt = &now
		s.activities[replaced.ID] = replaced
		delete(s.activities, activityKey(service.ID, replaced.Key))
	}
	expires := durationOrDefault(req.ExpiresInSeconds, 28800)
	stale := durationOrDefault(req.StaleAfterSeconds, 14400)
	activityID := "act_" + randomID()
	diagnostics, err := s.provider.StartActivity(ActivityPush{ActivityID: activityID, DeviceIDs: targets, CreatedAt: now})
	if err != nil {
		return Activity{}, false, err
	}
	activity := Activity{
		ID:                  activityID,
		ServiceID:           service.ID,
		Key:                 req.Key,
		DeviceIDs:           targets,
		Sequence:            0,
		Status:              "active",
		Delivered:           acceptedDeliveryCount(diagnostics),
		ProviderDiagnostics: diagnostics,
		CreatedAt:           now,
		ExpiresAt:           now.Add(expires),
		StaleAt:             now.Add(stale),
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
		s.activities[activityKey(service.ID, activity.Key)] = activity
	}
	if idemKey != "" {
		s.idempotency[tokenHash+":activity:"+idemKey] = IdempotencyRecord{Fingerprint: fingerprint, ActivityID: activity.ID, CreatedAt: now}
	}
	return activity, false, nil
}

func (s *Store) UpdateActivity(token, id string, req ActivityRequest) (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceForToken(token)
	if !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activityForService(service.ID, id)
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
	s.services[service.TokenHash] = service
	mergeActivity(&activity, req)
	activity.Sequence++
	now := time.Now().UTC()
	if req.StaleAfterSeconds != 0 {
		activity.StaleAt = now.Add(time.Duration(req.StaleAfterSeconds) * time.Second)
	}
	diagnostics, err := s.provider.UpdateActivity(ActivityPush{ActivityID: activity.ID, DeviceIDs: activity.DeviceIDs, CreatedAt: now})
	if err != nil {
		return Activity{}, err
	}
	activity.ProviderDiagnostics = append(activity.ProviderDiagnostics, diagnostics...)
	activity.Delivered = acceptedDeliveryCount(diagnostics)
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activityKey(service.ID, activity.Key)] = activity
	}
	return activity, nil
}

func (s *Store) Activities(token string) ([]Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceForToken(token)
	if !ok {
		return nil, ErrUnknownWebhook
	}
	seen := map[string]bool{}
	activities := make([]Activity, 0, len(s.activities))
	for _, activity := range s.activities {
		if activity.ServiceID != service.ID {
			continue
		}
		if seen[activity.ID] {
			continue
		}
		seen[activity.ID] = true
		if activity.EndedAt == nil && time.Now().UTC().After(activity.ExpiresAt) {
			activity.Status = "expired"
			s.activities[activity.ID] = activity
			if activity.Key != "" {
				s.activities[activityKey(service.ID, activity.Key)] = activity
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
	service, ok := s.serviceForToken(token)
	if !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activityForService(service.ID, id)
	if !ok {
		return Activity{}, ErrNotFound
	}
	if activity.EndedAt == nil && time.Now().UTC().After(activity.ExpiresAt) {
		activity.Status = "expired"
		s.activities[activity.ID] = activity
		if activity.Key != "" {
			s.activities[activityKey(service.ID, activity.Key)] = activity
		}
	}
	return activity, nil
}

func (s *Store) EndActivity(token, id string, req ActivityRequest) (Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceForToken(token)
	if !ok {
		return Activity{}, ErrUnknownWebhook
	}
	activity, ok := s.activityForService(service.ID, id)
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
	s.services[service.TokenHash] = service
	mergeActivity(&activity, req)
	activity.Sequence++
	now := time.Now().UTC()
	activity.Status = "ended"
	activity.EndedAt = &now
	dismissAt := now.Add(time.Duration(req.DismissAfterSeconds) * time.Second)
	activity.DismissAt = &dismissAt
	diagnostics, err := s.provider.EndActivity(ActivityPush{ActivityID: activity.ID, DeviceIDs: activity.DeviceIDs, CreatedAt: now})
	if err != nil {
		return Activity{}, err
	}
	activity.ProviderDiagnostics = append(activity.ProviderDiagnostics, diagnostics...)
	activity.Delivered = acceptedDeliveryCount(diagnostics)
	if activity.State.Status == "" {
		activity.State.Status = "Complete"
	}
	if activity.State.Symbol == "" {
		activity.State.Symbol = "success"
	}
	s.activities[activity.ID] = activity
	if activity.Key != "" {
		s.activities[activityKey(service.ID, activity.Key)] = activity
	}
	return activity, nil
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
