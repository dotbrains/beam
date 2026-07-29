package beam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type callbackDelivery struct {
	event   Event
	attempt CallbackAttempt
	url     string
	token   string
}

func (s *Store) DeliverDueCallbacks(ctx context.Context, client *http.Client, now time.Time) (int, error) {
	if client == nil {
		client = http.DefaultClient
	}
	deliveries := s.dueCallbackDeliveries(now)
	var firstErr error
	for _, delivery := range deliveries {
		statusCode, errText := deliverCallback(ctx, client, delivery)
		if errText != "" && firstErr == nil {
			firstErr = fmt.Errorf("delivering callback for %s attempt %d: %s", delivery.event.ID, delivery.attempt.Attempt, errText)
		}
		s.recordCallbackResult(delivery.event.ID, delivery.attempt.Attempt, statusCode, errText, time.Now().UTC())
	}
	return len(deliveries), firstErr
}

func (s *Store) dueCallbackDeliveries(now time.Time) []callbackDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveries := []callbackDelivery{}
	for _, event := range s.events {
		if event.Response == nil || event.Response.CallbackURL == "" {
			continue
		}
		for _, attempt := range event.Response.CallbackAttempts {
			if attempt.Status != "scheduled" || attempt.ScheduledAt.After(now) {
				continue
			}
			deliveries = append(deliveries, callbackDelivery{
				event:   event,
				attempt: attempt,
				url:     event.Response.CallbackURL,
				token:   event.Response.CallbackToken,
			})
		}
	}
	return deliveries
}

func deliverCallback(ctx context.Context, client *http.Client, delivery callbackDelivery) (int, string) {
	body, err := json.Marshal(map[string]any{
		"ok":      true,
		"eventId": delivery.event.ID,
		"event":   delivery.event,
	})
	if err != nil {
		return 0, err.Error()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.url, bytes.NewReader(body))
	if err != nil {
		return 0, "callback request creation failed"
	}
	req.Header.Set("Content-Type", "application/json")
	if delivery.token != "" {
		req.Header.Set("Authorization", "Bearer "+delivery.token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "callback request failed"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Status
	}
	return resp.StatusCode, ""
}

func (s *Store) recordCallbackResult(eventID string, attemptNumber, statusCode int, errText string, deliveredAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	event, ok := s.events[eventID]
	if !ok || event.Response == nil {
		return
	}
	for i, attempt := range event.Response.CallbackAttempts {
		if attempt.Attempt != attemptNumber || attempt.Status != "scheduled" {
			continue
		}
		attempt.Status = "delivered"
		if errText != "" {
			attempt.Status = "failed"
			attempt.Error = errText
		}
		attempt.StatusCode = statusCode
		attempt.DeliveredAt = &deliveredAt
		event.Response.CallbackAttempts[i] = attempt
		if errText == "" {
			cancelScheduledCallbackRetries(event.Response.CallbackAttempts[i+1:])
		}
		s.events[eventID] = event
		return
	}
}

func cancelScheduledCallbackRetries(attempts []CallbackAttempt) {
	for i, attempt := range attempts {
		if attempt.Status == "scheduled" {
			attempt.Status = "canceled"
			attempts[i] = attempt
		}
	}
}
