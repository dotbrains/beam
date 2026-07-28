package beam

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var ErrValidation = errors.New("validation failed")

func (req *ActivityRequest) UnmarshalJSON(data []byte) error {
	type activityRequest ActivityRequest
	var parsed activityRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*req = ActivityRequest(parsed)
	_, req.DetailSet = fields["detail"]
	_, req.ProgressSet = fields["progress"]
	return nil
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (e ValidationError) Error() string {
	return ErrValidation.Error()
}

func (e ValidationError) Unwrap() error {
	return ErrValidation
}

func validateIdempotencyKey(key string) error {
	if key == "" {
		return nil
	}
	if strings.TrimSpace(key) == "" {
		return ValidationError{Fields: []FieldError{{Field: "Idempotency-Key", Message: "must not be blank"}}}
	}
	if len(key) > 255 {
		return ValidationError{Fields: []FieldError{{Field: "Idempotency-Key", Message: "must be 255 characters or fewer"}}}
	}
	return nil
}

func validateNotification(req NotificationRequest) error {
	var fields []FieldError
	body := strings.TrimSpace(req.Body)
	if body == "" {
		fields = append(fields, FieldError{Field: "body", Message: "is required"})
	}
	if len(body) > 2000 {
		fields = append(fields, FieldError{Field: "body", Message: "must be 2,000 characters or fewer"})
	}
	if len(req.Title) > 80 {
		fields = append(fields, FieldError{Field: "title", Message: "must be 80 characters or fewer"})
	}
	if req.ImageURL != "" && !isPublicHTTPS(req.ImageURL) {
		fields = append(fields, FieldError{Field: "imageUrl", Message: "must be a public HTTPS URL"})
	}
	if req.URL != "" && !hasScheme(req.URL, "http", "https") {
		fields = append(fields, FieldError{Field: "url", Message: "must be an HTTP or HTTPS URL"})
	}
	fields = append(fields, validateDeviceIDList(req.DeviceIDs)...)
	if req.Response != nil {
		fields = append(fields, validateResponse(*req.Response)...)
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateServiceCreate(req ServiceCreateRequest) error {
	var fields []FieldError
	if strings.TrimSpace(req.Title) == "" {
		fields = append(fields, FieldError{Field: "title", Message: "is required"})
	}
	if len(req.Title) > 80 {
		fields = append(fields, FieldError{Field: "title", Message: "must be 80 characters or fewer"})
	}
	if req.ImageURL != "" && !isPublicHTTPS(req.ImageURL) {
		fields = append(fields, FieldError{Field: "imageUrl", Message: "must be a public HTTPS URL"})
	}
	if req.URL != "" && !hasScheme(req.URL, "http", "https") {
		fields = append(fields, FieldError{Field: "url", Message: "must be an HTTP or HTTPS URL"})
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateServiceUpdate(req ServiceUpdateRequest) error {
	var fields []FieldError
	if req.Title == nil && req.ImageURL == nil && req.URL == nil {
		fields = append(fields, FieldError{Field: "service", Message: "must include at least one update field"})
	}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			fields = append(fields, FieldError{Field: "title", Message: "must not be blank"})
		}
		if len(*req.Title) > 80 {
			fields = append(fields, FieldError{Field: "title", Message: "must be 80 characters or fewer"})
		}
	}
	if req.ImageURL != nil && *req.ImageURL != "" && !isPublicHTTPS(*req.ImageURL) {
		fields = append(fields, FieldError{Field: "imageUrl", Message: "must be a public HTTPS URL"})
	}
	if req.URL != nil && *req.URL != "" && !hasScheme(*req.URL, "http", "https") {
		fields = append(fields, FieldError{Field: "url", Message: "must be an HTTP or HTTPS URL"})
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateDeviceRegister(req DeviceRegisterRequest) error {
	var fields []FieldError
	if strings.TrimSpace(req.Name) == "" {
		fields = append(fields, FieldError{Field: "name", Message: "is required"})
	}
	if len(req.Name) > 80 {
		fields = append(fields, FieldError{Field: "name", Message: "must be 80 characters or fewer"})
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		fields = append(fields, FieldError{Field: "platform", Message: "is required"})
	} else if platform != "ios" {
		fields = append(fields, FieldError{Field: "platform", Message: "must be ios"})
	}
	token := strings.TrimSpace(req.PushToStartToken)
	if token != "" && (len(token) < 16 || len(token) > 512) {
		fields = append(fields, FieldError{Field: "pushToStartToken", Message: "must be 16..512 characters"})
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateDeviceIDList(ids []string) []FieldError {
	if ids == nil {
		return nil
	}
	if len(ids) == 0 {
		return []FieldError{{Field: "deviceIds", Message: "must contain at least one device"}}
	}
	fields := []FieldError{}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			fields = append(fields, FieldError{Field: "deviceIds", Message: "must not contain duplicate device " + id})
			continue
		}
		seen[id] = true
	}
	if len(ids) > 50 {
		fields = append(fields, FieldError{Field: "deviceIds", Message: "must contain 50 devices or fewer"})
	}
	return fields
}

func validateDeviceRouting(devices []Device, requestedIDs []string) error {
	if len(requestedIDs) == 0 {
		return nil
	}
	active := activeDeviceIDs(devices)
	var fields []FieldError
	for _, id := range requestedIDs {
		if !active[id] {
			fields = append(fields, FieldError{Field: "deviceIds", Message: "contains unknown or inactive device " + id})
		}
	}
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateResponse(req ResponseRequest) []FieldError {
	var fields []FieldError
	switch req.Type {
	case "approval", "yes_no", "text":
	case "":
		fields = append(fields, FieldError{Field: "response.type", Message: "is required"})
	default:
		fields = append(fields, FieldError{Field: "response.type", Message: "must be approval, yes_no, or text"})
	}
	if req.ExpiresInSeconds != 0 && (req.ExpiresInSeconds < 30 || req.ExpiresInSeconds > 86400) {
		fields = append(fields, FieldError{Field: "response.expiresInSeconds", Message: "must be between 30 and 86,400"})
	}
	if req.Callback != nil {
		if !isPublicHTTPS(req.Callback.URL) {
			fields = append(fields, FieldError{Field: "response.callback.url", Message: "must be a public HTTPS URL"})
		}
		callbackToken := strings.TrimSpace(req.Callback.Token)
		if callbackToken != req.Callback.Token || len(callbackToken) < 16 || len(callbackToken) > 512 {
			fields = append(fields, FieldError{Field: "response.callback.token", Message: "must be 16 to 512 non-space characters"})
		}
	}
	return fields
}

func validateActivityStart(req ActivityRequest) error {
	var fields []FieldError
	if strings.TrimSpace(req.Title) == "" {
		fields = append(fields, FieldError{Field: "title", Message: "is required"})
	}
	if strings.TrimSpace(req.Status) == "" {
		fields = append(fields, FieldError{Field: "status", Message: "is required"})
	}
	fields = append(fields, validateDeviceIDList(req.DeviceIDs)...)
	fields = append(fields, validateActivityFields(req, false)...)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateActivityUpdate(req ActivityRequest) error {
	var fields []FieldError
	if !hasActivityUpdateField(req) {
		fields = append(fields, FieldError{Field: "activity", Message: "must include at least one update field"})
	}
	if req.DismissAfterSeconds != nil {
		fields = append(fields, FieldError{Field: "dismissAfterSeconds", Message: "is only valid when ending an activity"})
	}
	fields = append(fields, validateActivityFields(req, true)...)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateActivityEnd(req ActivityRequest) error {
	var fields []FieldError
	if req.DismissAfterSeconds != nil && (*req.DismissAfterSeconds < 0 || *req.DismissAfterSeconds > 14400) {
		fields = append(fields, FieldError{Field: "dismissAfterSeconds", Message: "must be between 0 and 14,400"})
	}
	fields = append(fields, validateActivityFields(req, true)...)
	if len(fields) > 0 {
		return ValidationError{Fields: fields}
	}
	return nil
}

func validateActivityFields(req ActivityRequest, partial bool) []FieldError {
	var fields []FieldError
	if req.ExpiresInSeconds != nil && (*req.ExpiresInSeconds < 60 || *req.ExpiresInSeconds > 28800) {
		fields = append(fields, FieldError{Field: "expiresInSeconds", Message: "must be between 60 and 28,800"})
	}
	if req.StaleAfterSeconds != nil && (*req.StaleAfterSeconds < 0 || *req.StaleAfterSeconds > 28800) {
		fields = append(fields, FieldError{Field: "staleAfterSeconds", Message: "must be between 0 and 28,800"})
	}
	if req.Progress != nil && (*req.Progress < 0 || *req.Progress > 1) {
		fields = append(fields, FieldError{Field: "progress", Message: "must be between 0 and 1"})
	}
	if req.Symbol != "" && !oneOf(req.Symbol, "terminal", "code", "build", "success", "warning") {
		fields = append(fields, FieldError{Field: "symbol", Message: "must be terminal, code, build, success, or warning"})
	}
	if req.Style != "" && !oneOf(req.Style, "standard", "ring", "hero", "terminal", "steps") {
		fields = append(fields, FieldError{Field: "style", Message: "must be standard, ring, hero, terminal, or steps"})
	}
	if req.PrivacyMode != "" && !oneOf(req.PrivacyMode, "standard", "private") {
		fields = append(fields, FieldError{Field: "privacyMode", Message: "must be standard or private"})
	}
	if partial && len(req.DeviceIDs) > 0 {
		fields = append(fields, FieldError{Field: "deviceIds", Message: "is only valid when starting an activity"})
	}
	if !partial && req.DismissAfterSeconds != nil {
		fields = append(fields, FieldError{Field: "dismissAfterSeconds", Message: "is only valid when ending an activity"})
	}
	return fields
}

func hasActivityUpdateField(req ActivityRequest) bool {
	return req.Title != "" || req.Status != "" || req.DetailSet || req.Detail != nil || req.ProgressSet || req.Progress != nil ||
		req.Symbol != "" || req.AccentColor != "" || req.Style != "" || req.PrivacyMode != "" ||
		req.ExpiresInSeconds != nil || req.StaleAfterSeconds != nil
}

func hasScheme(raw string, schemes ...string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return true
		}
	}
	return false
}

func isPublicHTTPS(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return false
	}
	if parsed.User != nil {
		return false
	}
	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isPublicIP(ip)
	}
	return true
}

func isPublicIP(ip net.IP) bool {
	return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() && !ip.IsMulticast()
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func validationFields(err error) []FieldError {
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Fields
	}
	return []FieldError{{Field: "request", Message: fmt.Sprint(err)}}
}
