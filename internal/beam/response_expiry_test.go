package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExpiredPromptDoesNotScheduleCallbackAttempts(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"expire me",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Get(server.URL + "/hooks/dev_token/events/" + eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", resp.StatusCode)
	}
	var body struct {
		Event Event `json:"event"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Event.Response == nil || body.Event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", body.Event.Response)
	}
	if len(body.Event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", body.Event.Response.CallbackAttempts)
	}
}

func TestExpiredPromptRejectsLateResponseWithoutCallbacks(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"too late",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"approve"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("respond status = %d", resp.StatusCode)
	}

	event = store.events[eventID]
	if event.Response == nil || event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", event.Response)
	}
	if len(event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", event.Response.CallbackAttempts)
	}
}

func TestExpiredPromptRejectsCancelWithoutCallbacks(t *testing.T) {
	store := NewStore()
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	eventID := sendPrompt(t, server.URL, "dev_token", `{
		"body":"too late",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`)
	event := store.events[eventID]
	event.Response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	store.events[eventID] = event

	resp, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/cancel", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cancel status = %d", resp.StatusCode)
	}

	event = store.events[eventID]
	if event.Response == nil || event.Response.Status != "expired" {
		t.Fatalf("unexpected response: %#v", event.Response)
	}
	if len(event.Response.CallbackAttempts) != 0 {
		t.Fatalf("callback attempts = %#v", event.Response.CallbackAttempts)
	}
}

func TestPublicHTTPSValidationRejectsEmbeddedCredentials(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
		"body":"credentials",
		"imageUrl":"https://user:pass@example.com/avatar.png",
		"response":{
			"type":"approval",
			"callback":{"url":"https://user:pass@example.com/beam","token":"0123456789abcdef"}
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	var body struct {
		Fields []FieldError `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !hasFieldError(body.Fields, "imageUrl") || !hasFieldError(body.Fields, "response.callback.url") {
		t.Fatalf("field errors = %#v", body.Fields)
	}
}

func TestCallbackTokenRejectsWhitespace(t *testing.T) {
	for _, token := range []string{"                ", "01234567 9abcdef"} {
		handler := Handler(NewStore())

		req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
			"body":"credentials",
			"response":{
				"type":"approval",
				"callback":{"url":"https://callbacks.example.com/beam","token":`+strconv.Quote(token)+`}
			}
		}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("token %q status = %d", token, resp.Code)
		}
		assertFieldError(t, resp.Body, "response.callback.token")
	}
}

func TestCredentialLimitsCountUnicodeCharacters(t *testing.T) {
	handler := Handler(NewStore())
	server := httptest.NewServer(handler)
	defer server.Close()

	idempotencyReq := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{"body":"deploy"}`))
	idempotencyReq.Header.Set("Content-Type", "application/json")
	idempotencyReq.Header.Set("Idempotency-Key", strings.Repeat("火", 255))
	idempotencyResp := httptest.NewRecorder()
	handler.ServeHTTP(idempotencyResp, idempotencyReq)
	if idempotencyResp.Code != http.StatusOK {
		t.Fatalf("idempotency status = %d", idempotencyResp.Code)
	}

	callbackReq := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{"body":"credentials","response":{"type":"approval","callback":{"url":"https://callbacks.example.com/beam","token":"`+strings.Repeat("界", 16)+`"}}}`))
	callbackReq.Header.Set("Content-Type", "application/json")
	callbackResp := httptest.NewRecorder()
	handler.ServeHTTP(callbackResp, callbackReq)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("callback status = %d", callbackResp.Code)
	}

	created := createServiceForDeviceTest(t, server.URL)
	deviceResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewBufferString(`{"name":"Nick's iPhone","platform":"ios","pushToken":"`+strings.Repeat("火", 16)+`","pushToStartToken":"`+strings.Repeat("星", 16)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusCreated {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}
}

func TestCredentialLimitsRejectTooManyUnicodeCharacters(t *testing.T) {
	handler := Handler(NewStore())
	server := httptest.NewServer(handler)
	defer server.Close()

	idempotencyReq := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{"body":"deploy"}`))
	idempotencyReq.Header.Set("Content-Type", "application/json")
	idempotencyReq.Header.Set("Idempotency-Key", strings.Repeat("火", 256))
	idempotencyResp := httptest.NewRecorder()
	handler.ServeHTTP(idempotencyResp, idempotencyReq)
	if idempotencyResp.Code != http.StatusBadRequest {
		t.Fatalf("idempotency status = %d", idempotencyResp.Code)
	}
	assertFieldError(t, idempotencyResp.Body, "Idempotency-Key")

	callbackReq := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{"body":"credentials","response":{"type":"approval","callback":{"url":"https://callbacks.example.com/beam","token":"`+strings.Repeat("界", 513)+`"}}}`))
	callbackReq.Header.Set("Content-Type", "application/json")
	callbackResp := httptest.NewRecorder()
	handler.ServeHTTP(callbackResp, callbackReq)
	if callbackResp.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d", callbackResp.Code)
	}
	assertFieldError(t, callbackResp.Body, "response.callback.token")

	created := createServiceForDeviceTest(t, server.URL)
	deviceResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewBufferString(`{"name":"Nick's iPhone","platform":"ios","pushToken":"`+strings.Repeat("火", 513)+`","pushToStartToken":"`+strings.Repeat("星", 513)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}
	var body struct {
		Fields []FieldError `json:"fields"`
	}
	if err := json.NewDecoder(deviceResp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !hasFieldError(body.Fields, "pushToken") || !hasFieldError(body.Fields, "pushToStartToken") {
		t.Fatalf("field errors = %#v", body.Fields)
	}
}

func TestDeviceIDsRejectBlankEntries(t *testing.T) {
	handler := Handler(NewStore())

	req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
		"body":"route",
		"deviceIds":["dev_local","  "]
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	assertFieldError(t, resp.Body, "deviceIds")
}

func TestErrorResponsesIncludeBranchableCodes(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unknown webhook",
			method:     http.MethodPost,
			path:       "/hooks/missing",
			body:       `{"body":"deploy"}`,
			wantStatus: http.StatusNotFound,
			wantCode:   "unknown_webhook",
		},
		{
			name:       "invalid json",
			method:     http.MethodPost,
			path:       "/hooks/dev_token",
			body:       `{"body":`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "invalid payload",
			method:     http.MethodPost,
			path:       "/hooks/dev_token",
			body:       `{"body":"   "}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_payload",
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			path:       "/hooks/dev_token/events/missing",
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/healthz",
			body:       `{}`,
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   "method_not_allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			assertErrorCode(t, resp, tc.wantCode)
		})
	}
}

func TestExpiredActivityUpdateAndEndPersistTerminalState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "update", method: http.MethodPatch, path: "/hooks/dev_token/live-activities/deploy", body: `{"status":"Retrying"}`},
		{name: "end", method: http.MethodPost, path: "/hooks/dev_token/live-activities/deploy/end", body: `{"status":"Done"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore()
			server := httptest.NewServer(Handler(store))
			defer server.Close()

			startActivity(t, server.URL)
			activity := store.activities[activityKey("svc_dev", "deploy")]
			activity.ExpiresAt = time.Now().UTC().Add(-time.Second)
			store.activities[activity.ID] = activity
			store.activities[activityKey("svc_dev", activity.Key)] = activity

			req, err := http.NewRequest(tc.method, server.URL+tc.path, bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			assertErrorCode(t, resp, "terminal_activity")

			readResp, err := http.Get(server.URL + "/hooks/dev_token/live-activities/deploy")
			if err != nil {
				t.Fatal(err)
			}
			defer readResp.Body.Close()
			var body struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(readResp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Status != "expired" {
				t.Fatalf("status after terminal rejection = %q", body.Status)
			}
		})
	}
}

func TestManagementValidationCountsUnicodeCharacters(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	title := strings.Repeat("界", 80)
	createPayload, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		t.Fatal(err)
	}
	createResp, err := http.Post(server.URL+"/api/services", "application/json", bytes.NewReader(createPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	var created struct {
		Service PublicService `json:"service"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	patchPayload, err := json.Marshal(map[string]string{"title": strings.Repeat("星", 80)})
	if err != nil {
		t.Fatal(err)
	}
	patchReq, err := http.NewRequest(http.MethodPatch, server.URL+"/api/services/"+created.Service.ID, bytes.NewReader(patchPayload))
	if err != nil {
		t.Fatal(err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatal(err)
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d", patchResp.StatusCode)
	}

	devicePayload, err := json.Marshal(map[string]string{"name": strings.Repeat("火", 80), "platform": "ios"})
	if err != nil {
		t.Fatal(err)
	}
	deviceResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewReader(devicePayload))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusCreated {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}
}

func TestManagementValidationRejectsTooManyUnicodeCharacters(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	createPayload, err := json.Marshal(map[string]string{"title": strings.Repeat("界", 81)})
	if err != nil {
		t.Fatal(err)
	}
	createResp, err := http.Post(server.URL+"/api/services", "application/json", bytes.NewReader(createPayload))
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	assertFieldError(t, createResp.Body, "title")

	created := createServiceForDeviceTest(t, server.URL)
	devicePayload, err := json.Marshal(map[string]string{"name": strings.Repeat("火", 81), "platform": "ios"})
	if err != nil {
		t.Fatal(err)
	}
	deviceResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewReader(devicePayload))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}
	assertFieldError(t, deviceResp.Body, "name")
}

func hasFieldError(fields []FieldError, want string) bool {
	for _, field := range fields {
		if field.Field == want {
			return true
		}
	}
	return false
}
