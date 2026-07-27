package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthDeviceFlow(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	startResp, err := http.Post(server.URL+"/api/auth/device", "application/json", bytes.NewBufferString(`{"clientName":"CI","scopes":["notify"],"expiresInSeconds":3600}`))
	if err != nil {
		t.Fatal(err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusCreated {
		t.Fatalf("start status = %d", startResp.StatusCode)
	}
	var started struct {
		Device AuthDevice `json:"device"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	if started.Device.DeviceCode == "" || started.Device.UserCode == "" || started.Device.VerifyURL == "" {
		t.Fatalf("unexpected device response: %#v", started.Device)
	}

	pendingResp, err := http.Get(server.URL + "/api/auth/device/" + started.Device.DeviceCode + "/token")
	if err != nil {
		t.Fatal(err)
	}
	defer pendingResp.Body.Close()
	var pending map[string]any
	if err := json.NewDecoder(pendingResp.Body).Decode(&pending); err != nil {
		t.Fatal(err)
	}
	if pending["status"] != "pending" {
		t.Fatalf("pending status = %#v", pending["status"])
	}
	if _, ok := pending["token"]; ok {
		t.Fatalf("pending response leaked token: %#v", pending)
	}

	approveResp, err := http.Post(server.URL+"/api/auth/device/"+started.Device.UserCode+"/approve", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d", approveResp.StatusCode)
	}

	tokenResp, err := http.Get(server.URL + "/api/auth/device/" + started.Device.DeviceCode + "/token")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	var token map[string]any
	if err := json.NewDecoder(tokenResp.Body).Decode(&token); err != nil {
		t.Fatal(err)
	}
	if token["status"] != "approved" || token["token"] == "" {
		t.Fatalf("token response = %#v", token)
	}

	revokeResp, err := http.Post(server.URL+"/api/auth/revoke", "application/json", bytes.NewBufferString(`{"token":"`+token["token"].(string)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer revokeResp.Body.Close()
	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke status = %d", revokeResp.StatusCode)
	}

	revokedResp, err := http.Get(server.URL + "/api/auth/device/" + started.Device.DeviceCode + "/token")
	if err != nil {
		t.Fatal(err)
	}
	defer revokedResp.Body.Close()
	var revoked map[string]any
	if err := json.NewDecoder(revokedResp.Body).Decode(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked["status"] != "revoked" {
		t.Fatalf("revoked response = %#v", revoked)
	}
	if _, ok := revoked["token"]; ok {
		t.Fatalf("revoked response leaked token: %#v", revoked)
	}
}

func TestAuthConnectionsListAndRevokeAreTokenSafe(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	startResp, err := http.Post(server.URL+"/api/auth/device", "application/json", bytes.NewBufferString(`{"clientName":"Agent","scopes":["notify"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer startResp.Body.Close()
	var started struct {
		Device AuthDevice `json:"device"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	approveResp, err := http.Post(server.URL+"/api/auth/device/"+started.Device.UserCode+"/approve", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	approveResp.Body.Close()

	listResp, err := http.Get(server.URL + "/api/auth/connections")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var listBytes bytes.Buffer
	if _, err := listBytes.ReadFrom(listResp.Body); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(listBytes.Bytes(), []byte("beam_agent_")) || bytes.Contains(listBytes.Bytes(), []byte(started.Device.UserCode)) {
		t.Fatalf("connection list leaked credential data: %s", listBytes.String())
	}
	var list struct {
		Connections []PublicAuthDevice `json:"connections"`
	}
	if err := json.Unmarshal(listBytes.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Connections) != 1 || list.Connections[0].ID == "" || list.Connections[0].Status != "approved" {
		t.Fatalf("connections = %#v", list.Connections)
	}

	revokeResp, err := http.Post(server.URL+"/api/auth/connections/"+list.Connections[0].ID+"/revoke", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer revokeResp.Body.Close()
	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke status = %d", revokeResp.StatusCode)
	}
	var revoke struct {
		Connection PublicAuthDevice `json:"connection"`
	}
	if err := json.NewDecoder(revokeResp.Body).Decode(&revoke); err != nil {
		t.Fatal(err)
	}
	if revoke.Connection.Status != "revoked" {
		t.Fatalf("revoked connection = %#v", revoke.Connection)
	}
}

func TestDeviceRegisterRedactsPushToStartToken(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	created := createServiceForDeviceTest(t, server.URL)
	resp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewBufferString(`{
		"name":"Nick's iPhone",
		"platform":"ios",
		"pushToStartToken":"0123456789abcdef"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	var bodyBytes bytes.Buffer
	if _, err := bodyBytes.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bodyBytes.Bytes(), []byte("0123456789abcdef")) {
		t.Fatalf("device response leaked push token: %s", bodyBytes.String())
	}
	var body struct {
		Device PublicDevice `json:"device"`
	}
	if err := json.Unmarshal(bodyBytes.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Device.PushToStartTokenRegistered {
		t.Fatalf("push token registration not reported: %#v", body.Device)
	}

	listResp, err := http.Get(server.URL + "/api/services/" + created.Service.ID + "/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var listBytes bytes.Buffer
	if _, err := listBytes.ReadFrom(listResp.Body); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(listBytes.Bytes(), []byte("0123456789abcdef")) {
		t.Fatalf("device list leaked push token: %s", listBytes.String())
	}
}

func TestDeviceRegisterRejectsShortPushToStartToken(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	created := createServiceForDeviceTest(t, server.URL)
	resp, err := http.Post(server.URL+"/api/services/"+created.Service.ID+"/devices", "application/json", bytes.NewBufferString(`{
		"name":"Nick's iPhone",
		"platform":"ios",
		"pushToStartToken":"short"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	assertFieldError(t, resp.Body, "pushToStartToken")
}

func createServiceForDeviceTest(t *testing.T, baseURL string) ServiceCreateResponse {
	t.Helper()
	resp, err := http.Post(baseURL+"/api/services", "application/json", bytes.NewBufferString(`{"title":"Devices"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("service create status = %d", resp.StatusCode)
	}
	var created ServiceCreateResponse
	var body struct {
		Service PublicService `json:"service"`
		Token   string        `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	created.Service = body.Service
	created.Token = body.Token
	return created
}
