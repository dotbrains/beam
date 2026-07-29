package beam

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var callbackRetryDelays = []time.Duration{
	0,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	time.Hour,
}

type ResponseRequest struct {
	Type             string           `json:"type"`
	ExpiresInSeconds int              `json:"expiresInSeconds,omitempty"`
	CorrelationID    string           `json:"correlationId,omitempty"`
	Callback         *CallbackRequest `json:"callback,omitempty"`
}

type CallbackRequest struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type ResponseAnswerRequest struct {
	Action string `json:"action,omitempty"`
	Text   string `json:"text,omitempty"`
}

type ResponseState struct {
	Type             string            `json:"type"`
	Status           string            `json:"status"`
	Action           string            `json:"action,omitempty"`
	Text             string            `json:"text,omitempty"`
	CorrelationID    string            `json:"correlationId,omitempty"`
	ExpiresAt        time.Time         `json:"expiresAt"`
	RespondedAt      *time.Time        `json:"respondedAt,omitempty"`
	CallbackAttempts []CallbackAttempt `json:"callbackAttempts,omitempty"`
	CallbackURL      string            `json:"-"`
	CallbackToken    string            `json:"-"`
}

type CallbackAttempt struct {
	EventID     string     `json:"eventId"`
	Attempt     int        `json:"attempt"`
	Status      string     `json:"status"`
	ScheduledAt time.Time  `json:"scheduledAt"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
	StatusCode  int        `json:"statusCode,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (s *Store) RespondEvent(token, id string, req ResponseAnswerRequest) (Event, error) {
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
	now := time.Now().UTC()
	if now.After(event.Response.ExpiresAt) {
		event.Response.Status = "expired"
		s.events[id] = event
		return Event{}, ErrNotFound
	}
	status, action, text, err := responseSettlement(*event.Response, req)
	if err != nil {
		return Event{}, err
	}
	event.Response.Status = status
	event.Response.Action = action
	event.Response.Text = text
	event.Response.RespondedAt = &now
	event.Response.CallbackAttempts = scheduleCallbackAttempts(event, now)
	s.events[id] = event
	return event, nil
}

func responseSettlement(state ResponseState, req ResponseAnswerRequest) (string, string, string, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	text := strings.TrimSpace(req.Text)
	switch state.Type {
	case "approval":
		switch action {
		case "approve":
			return "approved", action, "", nil
		case "deny":
			return "denied", action, "", nil
		}
	case "yes_no":
		switch action {
		case "yes":
			return "yes", action, "", nil
		case "no":
			return "no", action, "", nil
		}
	case "text":
		if text != "" {
			return "replied", "", text, nil
		}
	}
	return "", "", "", ErrInvalidPayload
}

func scheduleCallbackAttempts(event Event, now time.Time) []CallbackAttempt {
	if event.Response == nil || event.Response.CallbackURL == "" {
		return nil
	}
	attempts := make([]CallbackAttempt, 0, len(callbackRetryDelays))
	for i, delay := range callbackRetryDelays {
		attempts = append(attempts, CallbackAttempt{
			EventID:     event.ID,
			Attempt:     i + 1,
			Status:      "scheduled",
			ScheduledAt: now.Add(delay),
		})
	}
	return attempts
}

func eventVisibleToService(event Event, service Service) bool {
	return event.ServiceID == "" || event.ServiceID == service.ID
}

func decodeResponseAnswer(w http.ResponseWriter, r *http.Request) (ResponseAnswerRequest, bool) {
	body := readBody(w, r)
	if body == nil {
		return ResponseAnswerRequest{}, false
	}
	var req ResponseAnswerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("Invalid JSON"))
		return ResponseAnswerRequest{}, false
	}
	return req, true
}
