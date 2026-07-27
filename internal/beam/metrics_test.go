package beam

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsEndpointReportsDeliveryAndCallbackCounts(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{
		"body":"metrics",
		"response":{
			"type":"approval",
			"callback":{"url":"https://callbacks.example.com/beam","token":"0123456789abcdef"}
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	body, eventID := decodeEventID(t, resp)
	if eventID == "" {
		t.Fatalf("missing event id: %s", body)
	}

	respond, err := http.Post(server.URL+"/hooks/dev_token/events/"+eventID+"/respond", "application/json", bytes.NewBufferString(`{"action":"approve"}`))
	if err != nil {
		t.Fatal(err)
	}
	respond.Body.Close()
	if respond.StatusCode != http.StatusOK {
		t.Fatalf("respond status = %d", respond.StatusCode)
	}

	metrics := readMetrics(t, server.URL)
	for _, want := range []string{
		"beam_deliveries_total 1",
		"beam_callback_attempts_scheduled_total 5",
	} {
		if !bytes.Contains(metrics, []byte(want)) {
			t.Fatalf("metrics missing %q: %s", want, metrics)
		}
	}
}

func TestNotificationWithNoRegisteredDevicesReturnsMessage(t *testing.T) {
	store := NewStore()
	store.RegisterService(Service{ID: "svc_empty", Token: "empty_token", Title: "Empty"})
	server := httptest.NewServer(Handler(store))
	defer server.Close()

	resp, err := http.Post(server.URL+"/hooks/empty_token", "application/json", bytes.NewBufferString(`{"body":"no devices"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notify status = %d", resp.StatusCode)
	}
	var body struct {
		OK        bool   `json:"ok"`
		EventID   string `json:"eventId"`
		Delivered int    `json:"delivered"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.EventID == "" || body.Delivered != 0 || body.Message == "" {
		t.Fatalf("unexpected no-device response: %#v", body)
	}
}

func decodeEventID(t *testing.T, resp *http.Response) ([]byte, string) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("notify status = %d body = %s", resp.StatusCode, body)
	}
	var payload struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return body, payload.EventID
}

func readMetrics(t *testing.T, baseURL string) []byte {
	t.Helper()
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
