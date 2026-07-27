package beam

import (
	"crypto/rand"
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
	Token      string    `json:"token,omitempty"`
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
