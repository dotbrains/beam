package beam

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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
