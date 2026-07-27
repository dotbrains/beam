package beam

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationEndpoint(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","title":"CI"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true || body["eventId"] == "" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestNotificationIdempotencyConflict(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	first, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token", bytes.NewBufferString(`{"body":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	first.Header.Set("Content-Type", "application/json")
	first.Header.Set("Idempotency-Key", "deploy-1")
	if resp, err := http.DefaultClient.Do(first); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}

	second, err := http.NewRequest(http.MethodPost, server.URL+"/hooks/dev_token", bytes.NewBufferString(`{"body":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	second.Header.Set("Content-Type", "application/json")
	second.Header.Set("Idempotency-Key", "deploy-1")
	resp, err := http.DefaultClient.Do(second)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLiveActivityLifecycle(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Deploy","status":"Building","key":"deploy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Testing","progress":0.6}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["sequence"].(float64) != 1 {
		t.Fatalf("unexpected sequence: %#v", body)
	}
}

func TestNotificationValidationRejectsPrivateImageURL(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","imageUrl":"https://127.0.0.1/avatar.png"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "imageUrl")
}

func TestNotificationValidationRejectsInvalidResponseExpiry(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"approve deploy","response":{"type":"approval","expiresInSeconds":10}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "response.expiresInSeconds")
}

func TestLiveActivityRejectsEmptyUpdate(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "activity")
}

func TestLiveActivitySequenceConflict(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Testing","ifSequence":2}`))
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
}

func TestLiveActivityRejectsUpdateAfterEnd(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	resp, err := http.Post(server.URL+"/hooks/dev_token/live-activities/deploy/end", "application/json", bytes.NewBufferString(`{"status":"Done"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("end status = %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPatch, server.URL+"/hooks/dev_token/live-activities/deploy", bytes.NewBufferString(`{"status":"Again"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLiveActivityListDeduplicatesKeys(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()
	startActivity(t, server.URL)

	resp, err := http.Get(server.URL + "/hooks/dev_token/live-activities")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		OK         bool             `json:"ok"`
		Activities []map[string]any `json:"activities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK {
		t.Fatalf("ok = false")
	}
	if len(body.Activities) != 1 {
		t.Fatalf("activities = %#v", body.Activities)
	}
	if body.Activities[0]["key"] != "deploy" {
		t.Fatalf("activity key = %#v", body.Activities[0]["key"])
	}
}

func TestServiceLifecycleRoutes(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	createResp, err := http.Post(server.URL+"/api/services", "application/json", bytes.NewBufferString(`{"title":"CI","url":"https://ci.example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", createResp.StatusCode)
	}
	var created struct {
		Service PublicService `json:"service"`
		Token   string        `json:"token"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Service.ID == "" || created.Token == "" {
		t.Fatalf("unexpected create response: %#v", created)
	}

	listResp, err := http.Get(server.URL + "/api/services")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	listBytes, err := io.ReadAll(listResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(listBytes, []byte(created.Token)) {
		t.Fatalf("list leaked service token: %s", string(listBytes))
	}

	patchReq, err := http.NewRequest(http.MethodPatch, server.URL+"/api/services/"+created.Service.ID, bytes.NewBufferString(`{"title":"Deploys"}`))
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

	rotateResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/rotate-token", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rotateResp.Body.Close()
	if rotateResp.StatusCode != http.StatusOK {
		t.Fatalf("rotate status = %d", rotateResp.StatusCode)
	}
	var rotated struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rotateResp.Body).Decode(&rotated); err != nil {
		t.Fatal(err)
	}
	if rotated.Token == "" || rotated.Token == created.Token {
		t.Fatalf("unexpected rotated token: %#v", rotated)
	}

	oldWebhookResp, err := http.Post(server.URL+"/hooks/"+created.Token, "application/json", bytes.NewBufferString(`{"body":"should fail"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer oldWebhookResp.Body.Close()
	if oldWebhookResp.StatusCode != http.StatusNotFound {
		t.Fatalf("old token status = %d", oldWebhookResp.StatusCode)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/services/"+created.Service.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatal(err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", deleteResp.StatusCode)
	}
}

func TestDeviceRoutesAndRouting(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	createResp, err := http.Post(server.URL+"/api/services", "application/json", bytes.NewBufferString(`{"title":"CI"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	var created struct {
		Service PublicService `json:"service"`
		Token   string        `json:"token"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	deviceResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewBufferString(`{"name":"Nick's iPhone","platform":"ios"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusCreated {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}
	var registered struct {
		Device Device `json:"device"`
	}
	if err := json.NewDecoder(deviceResp.Body).Decode(&registered); err != nil {
		t.Fatal(err)
	}
	if registered.Device.ID == "" || !registered.Device.Active {
		t.Fatalf("unexpected device: %#v", registered.Device)
	}

	notifyResp, err := http.Post(server.URL+"/hooks/"+created.Token, "application/json", bytes.NewBufferString(`{"body":"routed","deviceIds":["`+registered.Device.ID+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer notifyResp.Body.Close()
	if notifyResp.StatusCode != http.StatusOK {
		t.Fatalf("notify status = %d", notifyResp.StatusCode)
	}
	var notifyBody map[string]any
	if err := json.NewDecoder(notifyResp.Body).Decode(&notifyBody); err != nil {
		t.Fatal(err)
	}
	if notifyBody["delivered"].(float64) != 1 {
		t.Fatalf("unexpected delivered count: %#v", notifyBody)
	}

	inactiveResp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices/"+registered.Device.ID+"/deactivate", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inactiveResp.Body.Close()
	if inactiveResp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate status = %d", inactiveResp.StatusCode)
	}

	badRouteResp, err := http.Post(server.URL+"/hooks/"+created.Token, "application/json", bytes.NewBufferString(`{"body":"routed","deviceIds":["`+registered.Device.ID+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer badRouteResp.Body.Close()
	if badRouteResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad route status = %d", badRouteResp.StatusCode)
	}
	assertFieldError(t, badRouteResp.Body, "deviceIds")
}

func startActivity(t *testing.T, baseURL string) {
	t.Helper()
	resp, err := http.Post(baseURL+"/hooks/dev_token/live-activities", "application/json", bytes.NewBufferString(`{"title":"Deploy","status":"Building","key":"deploy"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", resp.StatusCode)
	}
}

func assertFieldError(t *testing.T, body io.Reader, field string) {
	t.Helper()
	var payload struct {
		Fields []FieldError `json:"fields"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	for _, item := range payload.Fields {
		if item.Field == field {
			return
		}
	}
	t.Fatalf("missing field error %q in %#v", field, payload.Fields)
}
