package beam

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotificationValidationAllowsOnlyHTTPTapURLs(t *testing.T) {
	server := httptest.NewServer(Handler(NewStore()))
	defer server.Close()

	for _, rawURL := range []string{"http://example.com/build", "https://example.com/build"} {
		resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","url":"`+rawURL+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", rawURL, resp.StatusCode)
		}
	}

	for _, rawURL := range []string{"ftp://example.com/build", "javascript:alert(1)"} {
		resp, err := http.Post(server.URL+"/hooks/dev_token", "application/json", bytes.NewBufferString(`{"body":"build passed","url":"`+rawURL+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status = %d", rawURL, resp.StatusCode)
		}
		assertFieldError(t, resp.Body, "url")
	}
}

func TestCallbackTokenValidationEnforcesDocumentedBounds(t *testing.T) {
	for _, token := range []string{strings.Repeat("a", 16), strings.Repeat("b", 512)} {
		handler := Handler(NewStore())

		req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
			"body":"credentials",
			"response":{
				"type":"approval",
				"callback":{"url":"https://callbacks.example.com/beam","token":"`+token+`"}
			}
		}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("token length %d status = %d", len(token), resp.Code)
		}
	}

	for _, token := range []string{strings.Repeat("a", 15), strings.Repeat("b", 513)} {
		handler := Handler(NewStore())

		req := httptest.NewRequest(http.MethodPost, "/hooks/dev_token", bytes.NewBufferString(`{
			"body":"credentials",
			"response":{
				"type":"approval",
				"callback":{"url":"https://callbacks.example.com/beam","token":"`+token+`"}
			}
		}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("token length %d status = %d", len(token), resp.Code)
		}
		assertFieldError(t, resp.Body, "response.callback.token")
	}
}
