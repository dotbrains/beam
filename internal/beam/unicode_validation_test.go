package beam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceValidationCountsUnicodeCharacters(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	title := strings.Repeat("界", 80)
	createResp, err := postJSON(server.URL+"/api/services", map[string]string{"title": title})
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

	updateTitle := strings.Repeat("星", 80)
	patchReq, err := jsonRequest(http.MethodPatch, server.URL+"/api/services/"+created.Service.ID, map[string]string{"title": updateTitle})
	if err != nil {
		t.Fatal(err)
	}
	patchResp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatal(err)
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d", patchResp.StatusCode)
	}

	tooLongResp, err := postJSON(server.URL+"/api/services", map[string]string{"title": strings.Repeat("火", 81)})
	if err != nil {
		t.Fatal(err)
	}
	defer tooLongResp.Body.Close()
	if tooLongResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("too long create status = %d", tooLongResp.StatusCode)
	}
	assertFieldError(t, tooLongResp.Body, "title")
}

func TestDeviceValidationCountsUnicodeCharacters(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	created := createServiceForDeviceTest(t, server.URL)
	deviceResp, err := postJSON(server.URL+"/api/services/"+created.Service.ID+"/devices", map[string]string{
		"name":     strings.Repeat("星", 80),
		"platform": "ios",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer deviceResp.Body.Close()
	if deviceResp.StatusCode != http.StatusCreated {
		t.Fatalf("device status = %d", deviceResp.StatusCode)
	}

	tooLongResp, err := postJSON(server.URL+"/api/services/"+created.Service.ID+"/devices", map[string]string{
		"name":     strings.Repeat("星", 81),
		"platform": "ios",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer tooLongResp.Body.Close()
	if tooLongResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("too long device status = %d", tooLongResp.StatusCode)
	}
	assertFieldError(t, tooLongResp.Body, "name")
}

func postJSON(url string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post(url, "application/json", bytes.NewReader(body))
}

func jsonRequest(method, url string, payload any) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
