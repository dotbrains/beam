package beam

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"
)

type AuthDeviceRequest struct {
	ClientName       string   `json:"clientName,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	ExpiresInSeconds int      `json:"expiresInSeconds,omitempty"`
}

type AuthDevice struct {
	DeviceCode string    `json:"deviceCode"`
	UserCode   string    `json:"userCode"`
	VerifyURL  string    `json:"verifyUrl"`
	ClientName string    `json:"clientName,omitempty"`
	Scopes     []string  `json:"scopes,omitempty"`
	Token      string    `json:"-"`
	TokenHash  string    `json:"tokenHash,omitempty"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

type PublicAuthDevice struct {
	ID         string    `json:"id"`
	ClientName string    `json:"clientName,omitempty"`
	Scopes     []string  `json:"scopes,omitempty"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *Store) StartAuthDevice(req AuthDeviceRequest, verifyBaseURL string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	expires := durationOrDefault(req.ExpiresInSeconds, 900)
	device := AuthDevice{
		DeviceCode: "adc_" + randomID(),
		UserCode:   userCode(),
		VerifyURL:  strings.TrimRight(verifyBaseURL, "/") + "/auth/verify",
		ClientName: strings.TrimSpace(req.ClientName),
		Scopes:     append([]string(nil), req.Scopes...),
		Status:     "pending",
		ExpiresAt:  now.Add(expires),
		CreatedAt:  now,
	}
	s.authDevices[device.DeviceCode] = device
	return device, nil
}

func (s *Store) AuthDevices() []PublicAuthDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	devices := make([]PublicAuthDevice, 0, len(s.authDevices))
	for _, device := range s.authDevices {
		devices = append(devices, device.Public())
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})
	return devices
}

func (s *Store) ApproveAuthDevice(userCode string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, device := range s.authDevices {
		if device.UserCode != strings.ToUpper(strings.TrimSpace(userCode)) {
			continue
		}
		if time.Now().UTC().After(device.ExpiresAt) {
			device.Status = "expired"
			s.authDevices[device.DeviceCode] = device
			return device, ErrNotFound
		}
		if device.Status != "pending" {
			return device, ErrConflict
		}
		device.Status = "approved"
		device.Token = "beam_agent_" + randomID()
		device.TokenHash = hashToken(device.Token)
		s.authDevices[device.DeviceCode] = device
		return device, nil
	}
	return AuthDevice{}, ErrNotFound
}

func (s *Store) AuthDeviceToken(deviceCode string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.authDevices[deviceCode]
	if !ok {
		return AuthDevice{}, ErrNotFound
	}
	if time.Now().UTC().After(device.ExpiresAt) {
		device.Status = "expired"
		s.authDevices[device.DeviceCode] = device
		return device, nil
	}
	return device, nil
}

func (s *Store) AuthDeviceForToken(token string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthDevice{}, ErrUnauthorized
	}
	for _, device := range s.authDevices {
		if !authDeviceTokenMatches(device, token) {
			continue
		}
		if device.Status != "approved" || time.Now().UTC().After(device.ExpiresAt) {
			return AuthDevice{}, ErrUnauthorized
		}
		return device, nil
	}
	return AuthDevice{}, ErrUnauthorized
}

func (s *Store) RevokeAuthDevice(deviceCode string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.authDevices[strings.TrimSpace(deviceCode)]
	if !ok {
		return AuthDevice{}, ErrNotFound
	}
	device.Status = "revoked"
	device.Token = ""
	s.authDevices[device.DeviceCode] = device
	return device, nil
}

func (s *Store) RevokeAuthToken(token string) (AuthDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthDevice{}, ErrNotFound
	}
	for _, device := range s.authDevices {
		if !authDeviceTokenMatches(device, token) {
			continue
		}
		device.Status = "revoked"
		device.Token = ""
		s.authDevices[device.DeviceCode] = device
		return device, nil
	}
	return AuthDevice{}, ErrNotFound
}

func authDeviceTokenMatches(device AuthDevice, token string) bool {
	if device.TokenHash != "" {
		return device.TokenHash == hashToken(token)
	}
	return device.Token == token
}

func (d AuthDevice) Public() PublicAuthDevice {
	return PublicAuthDevice{
		ID:         d.DeviceCode,
		ClientName: d.ClientName,
		Scopes:     append([]string(nil), d.Scopes...),
		Status:     d.Status,
		ExpiresAt:  d.ExpiresAt,
		CreatedAt:  d.CreatedAt,
	}
}

func handleAuthDevice(store Backend, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
		return
	}
	body := readBody(w, r)
	if body == nil {
		return
	}
	var req AuthDeviceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("Invalid JSON"))
		return
	}
	device, err := store.StartAuthDevice(req, publicBaseURL(r))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "device": device})
}

func handleAuthDevicePath(store Backend, w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/auth/device/"), "/")
	if len(parts) == 2 && parts[1] == "token" && r.Method == http.MethodGet {
		device, err := store.AuthDeviceToken(parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, authDeviceTokenResponse(device))
		return
	}
	if len(parts) == 2 && parts[1] == "approve" && r.Method == http.MethodPost {
		device, err := store.ApproveAuthDevice(parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, authDeviceTokenResponse(device))
		return
	}
	writeJSON(w, http.StatusNotFound, errorBody("Not found"))
}

func handleAuthRevoke(store Backend, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
		return
	}
	token := bearerToken(r.Header.Get("Authorization"))
	body := readBody(w, r)
	if body == nil {
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody("Invalid JSON"))
			return
		}
	}
	if req.Token != "" {
		token = req.Token
	}
	device, err := store.RevokeAuthToken(token)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"credential": map[string]any{
			"status":     device.Status,
			"clientName": device.ClientName,
			"scopes":     device.Scopes,
		},
	})
}

func handleAuthConnections(store Backend, w http.ResponseWriter, r *http.Request) {
	if !authorizeBearerScope(store, w, r, "auth") {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusNotFound, errorBody("Not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connections": store.AuthDevices()})
}

func handleAuthConnection(store Backend, w http.ResponseWriter, r *http.Request) {
	if !authorizeBearerScope(store, w, r, "auth") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/auth/connections/"), "/")
	if len(parts) == 2 && parts[1] == "revoke" && r.Method == http.MethodPost {
		device, err := store.RevokeAuthDevice(parts[0])
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connection": device.Public()})
		return
	}
	writeJSON(w, http.StatusNotFound, errorBody("Not found"))
}

func userCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.ToUpper(randomID()[:4] + "-" + randomID()[:4])
	}
	var out strings.Builder
	for i, value := range buf {
		if i == 4 {
			out.WriteByte('-')
		}
		out.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return out.String()
}
