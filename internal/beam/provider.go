package beam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	Targets   []PushTarget
	CreatedAt time.Time
}

type ActivityPush struct {
	ActivityID string
	DeviceIDs  []string
	Targets    []PushTarget
	CreatedAt  time.Time
}

type PushTarget struct {
	DeviceID         string `json:"deviceId"`
	PushToken        string `json:"pushToken,omitempty"`
	PushToStartToken string `json:"pushToStartToken,omitempty"`
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

type HTTPPushProvider struct {
	Endpoint string
	Token    string
	Client   *http.Client
}

type providerDeliveryRequest struct {
	Operation  string       `json:"operation"`
	EventID    string       `json:"eventId,omitempty"`
	ActivityID string       `json:"activityId,omitempty"`
	DeviceIDs  []string     `json:"deviceIds,omitempty"`
	Devices    []PushTarget `json:"devices,omitempty"`
	CreatedAt  time.Time    `json:"createdAt"`
}

type providerDeliveryResponse struct {
	Diagnostics []ProviderDiagnostic `json:"diagnostics"`
}

func (p HTTPPushProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	return p.post(providerDeliveryRequest{
		Operation: "notification",
		EventID:   req.EventID,
		DeviceIDs: req.DeviceIDs,
		Devices:   req.Targets,
		CreatedAt: req.CreatedAt,
	})
}

func (p HTTPPushProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.post(activityProviderRequest("activity_start", req))
}

func (p HTTPPushProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.post(activityProviderRequest("activity_update", req))
}

func (p HTTPPushProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return p.post(activityProviderRequest("activity_end", req))
}

func activityProviderRequest(operation string, req ActivityPush) providerDeliveryRequest {
	return providerDeliveryRequest{
		Operation:  operation,
		ActivityID: req.ActivityID,
		DeviceIDs:  req.DeviceIDs,
		Devices:    req.Targets,
		CreatedAt:  req.CreatedAt,
	}
}

func providerTargets(devices []Device, targetIDs []string, includePushToStart bool) []PushTarget {
	if len(targetIDs) == 0 {
		return nil
	}
	devicesByID := map[string]Device{}
	for _, device := range devices {
		devicesByID[device.ID] = device
	}
	targets := make([]PushTarget, 0, len(targetIDs))
	for _, id := range targetIDs {
		device, ok := devicesByID[id]
		if !ok {
			continue
		}
		target := PushTarget{DeviceID: id, PushToken: strings.TrimSpace(device.PushToken)}
		if includePushToStart {
			target.PushToStartToken = strings.TrimSpace(device.PushToStartToken)
		}
		targets = append(targets, target)
	}
	return targets
}

func (p HTTPPushProvider) post(payload providerDeliveryRequest) ([]ProviderDiagnostic, error) {
	endpoint := strings.TrimSpace(p.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("%w: missing provider endpoint", ErrProviderFailure)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: marshaling provider request", ErrProviderFailure)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: creating provider request", ErrProviderFailure)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: provider request failed", ErrProviderFailure)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: provider returned %s", ErrProviderFailure, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading provider response", ErrProviderFailure)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return deliveryDiagnostics("http_"+payload.Operation, payload.DeviceIDs, len(payload.DeviceIDs) == 0, payload.CreatedAt), nil
	}
	var decoded providerDeliveryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("%w: decoding provider response", ErrProviderFailure)
	}
	if len(decoded.Diagnostics) == 0 {
		return deliveryDiagnostics("http_"+payload.Operation, payload.DeviceIDs, len(payload.DeviceIDs) == 0, payload.CreatedAt), nil
	}
	return decoded.Diagnostics, nil
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
	providerDevices := providerTargets(service.Devices, targets, false)
	diagnostics, err := s.provider.SendNotification(PushNotification{EventID: eventID, DeviceIDs: targets, Targets: providerDevices, CreatedAt: now})
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

func (s *Store) StartActivity(token string, req ActivityRequest, idemKey, fingerprint string) (Activity, bool, error) {
	s.mu.Lock()
	service, ok := s.serviceForToken(token)
	if !ok {
		s.mu.Unlock()
		return Activity{}, false, ErrUnknownWebhook
	}
	tokenHash := hashToken(token)
	if err := validateActivityStart(req); err != nil {
		s.mu.Unlock()
		return Activity{}, false, err
	}
	if len(req.DeviceIDs) > 0 && !service.Limits.DeviceRouting {
		s.mu.Unlock()
		return Activity{}, false, ErrPaymentRequired
	}
	if err := validateDeviceRouting(service.Devices, req.DeviceIDs); err != nil {
		s.mu.Unlock()
		return Activity{}, false, err
	}
	now := time.Now().UTC()
	recordKey := ""
	pruneIdempotencyRecords(s.idempotency, now)
	if idemKey != "" {
		recordKey = tokenHash + ":activity:" + idemKey
		if record, ok := s.idempotency[recordKey]; ok {
			if record.Fingerprint != fingerprint {
				s.mu.Unlock()
				return Activity{}, false, ErrIdempotencyConflict
			}
			activity, ok := s.activities[record.ActivityID]
			if !ok || activity.Status == "processing" {
				s.mu.Unlock()
				return Activity{}, false, ErrPendingRequest
			}
			s.mu.Unlock()
			return activity, true, nil
		}
	}
	targets := activityTargetDeviceIDs(service.Devices, req.DeviceIDs)
	replaced := map[string]Activity{}
	for _, existing := range s.activities {
		if existing.ServiceID != service.ID {
			continue
		}
		if existing.EndedAt != nil || now.After(existing.ExpiresAt) {
			continue
		}
		if existing.ID != "" && (existing.Key == req.Key && req.Key != "" || overlaps(existing.DeviceIDs, targets)) {
			if !req.Replace {
				s.mu.Unlock()
				return Activity{}, false, ErrConflict
			}
			replaced[existing.ID] = existing
		}
	}
	key := replacementKey(req.Key, replaced)
	account := s.accountForService(service)
	limit, limited := consumeOperation(&service, account, now)
	if limited {
		s.mu.Unlock()
		return Activity{}, false, limit
	}
	s.storeAccount(account)
	s.services[service.TokenHash] = service
	for _, replaced := range replaced {
		replaced.Status = "ended"
		replaced.Sequence++
		replaced.EndedAt = &now
		s.activities[replaced.ID] = replaced
		delete(s.activities, activityKey(service.ID, replaced.Key))
	}
	expires := durationOrDefault(optionalInt(req.ExpiresInSeconds), 28800)
	stale := optionalDurationOrDefault(req.StaleAfterSeconds, 14400)
	activityID := "act_" + randomID()
	processing := Activity{ID: activityID, ServiceID: service.ID, Key: key, DeviceIDs: targets, Status: "processing", CreatedAt: now, ExpiresAt: now.Add(expires), StaleAt: now.Add(stale)}
	s.activities[activityID] = processing
	if key != "" {
		s.activities[activityKey(service.ID, key)] = processing
	}
	if recordKey != "" {
		s.idempotency[recordKey] = IdempotencyRecord{Fingerprint: fingerprint, ActivityID: activityID, CreatedAt: now}
	}
	s.mu.Unlock()
	providerDevices := providerTargets(service.Devices, targets, true)
	diagnostics, err := s.provider.StartActivity(ActivityPush{ActivityID: activityID, DeviceIDs: targets, Targets: providerDevices, CreatedAt: now})
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if recordKey != "" {
			delete(s.idempotency, recordKey)
		}
		s.activities[activityID] = Activity{
			ID:                  activityID,
			ServiceID:           service.ID,
			Key:                 key,
			DeviceIDs:           targets,
			Status:              "failed",
			ProviderDiagnostics: []ProviderDiagnostic{providerFailureDiagnostic("activity_start", now)},
			CreatedAt:           now,
			ExpiresAt:           now.Add(expires),
			StaleAt:             now.Add(stale),
			State:               ActivityState{Title: req.Title, Status: req.Status},
		}
		return Activity{}, false, err
	}
	activity := Activity{
		ID:                  activityID,
		ServiceID:           service.ID,
		Key:                 key,
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
	return activity, false, nil
}
