package beam

import (
	"strings"
	"time"
)

type PushProvider interface {
	SendNotification(req PushNotification) ([]ProviderDiagnostic, error)
	StartActivity(req ActivityPush) ([]ProviderDiagnostic, error)
	UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error)
	EndActivity(req ActivityPush) ([]ProviderDiagnostic, error)
}

type PushNotification struct {
	EventID   string
	DeviceIDs []string
	CreatedAt time.Time
}

type ActivityPush struct {
	ActivityID string
	DeviceIDs  []string
	CreatedAt  time.Time
}

type ProviderDiagnostic struct {
	Provider  string    `json:"provider"`
	Operation string    `json:"operation"`
	DeviceID  string    `json:"deviceId,omitempty"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type LocalPushProvider struct{}

func (LocalPushProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("notification", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_start", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_update", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_end", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func acceptedDeliveryCount(diagnostics []ProviderDiagnostic) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Status == "accepted" {
			count++
		}
	}
	return count
}

func deliveryDiagnostics(operation string, targetIDs []string, noTarget bool, now time.Time) []ProviderDiagnostic {
	if noTarget {
		return []ProviderDiagnostic{{
			Provider:  "local",
			Operation: operation,
			Status:    "skipped",
			Reason:    "no_active_device",
			CreatedAt: now,
		}}
	}
	diagnostics := make([]ProviderDiagnostic, 0, len(targetIDs))
	for _, id := range targetIDs {
		diagnostics = append(diagnostics, ProviderDiagnostic{
			Provider:  "local",
			Operation: operation,
			DeviceID:  id,
			Status:    "accepted",
			CreatedAt: now,
		})
	}
	return diagnostics
}

func providerFailureDiagnostic(operation string, now time.Time) ProviderDiagnostic {
	return ProviderDiagnostic{
		Provider:  "unknown",
		Operation: operation,
		Status:    "failed",
		Reason:    "provider_failure",
		CreatedAt: now,
	}
}

func (s *Store) SendNotification(token string, req NotificationRequest, idemKey, fingerprint string) (Event, bool, error) {
	s.mu.Lock()
	service, ok := s.serviceForToken(token)
	if !ok {
		s.mu.Unlock()
		return Event{}, false, ErrUnknownWebhook
	}
	tokenHash := hashToken(token)
	if err := validateNotification(req); err != nil {
		s.mu.Unlock()
		return Event{}, false, err
	}
	if len(req.DeviceIDs) > 0 && !service.Limits.DeviceRouting {
		s.mu.Unlock()
		return Event{}, false, ErrPaymentRequired
	}
	if err := validateDeviceRouting(service.Devices, req.DeviceIDs); err != nil {
		s.mu.Unlock()
		return Event{}, false, err
	}
	now := time.Now().UTC()
	recordKey := ""
	pruneIdempotencyRecords(s.idempotency, now)
	if idemKey != "" {
		recordKey = tokenHash + ":" + idemKey
		if record, ok := s.idempotency[recordKey]; ok {
			if record.Fingerprint != fingerprint {
				s.mu.Unlock()
				return Event{}, false, ErrIdempotencyConflict
			}
			event, ok := s.events[record.EventID]
			if !ok {
				s.mu.Unlock()
				return Event{}, false, ErrPendingRequest
			}
			s.mu.Unlock()
			return event, true, nil
		}
	}
	account := s.accountForService(service)
	limit, limited := consumeOperation(&service, account, now)
	if limited {
		s.mu.Unlock()
		return Event{}, false, limit
	}
	s.storeAccount(account)
	s.services[service.TokenHash] = service
	title := firstNonBlank(req.Title, service.Title, "Beam")
	targets := activityTargetDeviceIDs(service.Devices, req.DeviceIDs)
	eventID := "evt_" + randomID()
	if recordKey != "" {
		s.idempotency[recordKey] = IdempotencyRecord{Fingerprint: fingerprint, EventID: eventID, CreatedAt: now}
	}
	s.mu.Unlock()
	diagnostics, err := s.provider.SendNotification(PushNotification{EventID: eventID, DeviceIDs: targets, CreatedAt: now})
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if recordKey != "" {
			delete(s.idempotency, recordKey)
		}
		s.events[eventID] = Event{
			ID:                  eventID,
			ServiceID:           service.ID,
			Title:               title,
			Body:                strings.TrimSpace(req.Body),
			ImageURL:            firstNonEmpty(req.ImageURL, service.ImageURL),
			URL:                 firstNonEmpty(req.URL, service.URL),
			ProviderDiagnostics: []ProviderDiagnostic{providerFailureDiagnostic("notification", now)},
			CreatedAt:           now,
		}
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
	return event, false, nil
}
