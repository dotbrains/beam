package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dotbrains/beam/internal/config"
)

func apiClient() (*config.Config, *http.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	return cfg, &http.Client{Timeout: 15 * time.Second}, nil
}

func postJSON(client *http.Client, url string, payload any, idempotencyKey string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return do(req, client)
}

func patchJSON(client *http.Client, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req, client)
}

func deleteJSON(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return do(req, client)
}

func getJSON(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return do(req, client)
}

func do(req *http.Request, client *http.Client) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("api returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func hookURL(cfg *config.Config, suffix string) string {
	return strings.TrimRight(cfg.APIURL, "/") + "/hooks/" + cfg.Token + suffix
}

func apiURL(cfg *config.Config, suffix string) string {
	return strings.TrimRight(cfg.APIURL, "/") + suffix
}
