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
