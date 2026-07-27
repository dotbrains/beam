package beam

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Service struct {
	ID            string
	Token         string
	Title         string
	ImageURL      string
	URL           string
	Devices       []Device
	Limits        ServiceLimits
	Usage         ServiceUsage
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevokedTokens []string
}

type ServiceLimits struct {
	RequestsPerMinute  int  `json:"requestsPerMinute,omitempty"`
	MonthlyOperations  int  `json:"monthlyOperations,omitempty"`
	DeviceRouting      bool `json:"deviceRouting,omitempty"`
	PermissiveDefaults bool `json:"permissiveDefaults,omitempty"`
}

type ServiceUsage struct {
	MinuteWindowStartedAt time.Time `json:"minuteWindowStartedAt,omitempty"`
	MinuteOperations      int       `json:"minuteOperations,omitempty"`
	MonthWindow           string    `json:"monthWindow,omitempty"`
	MonthOperations       int       `json:"monthOperations,omitempty"`
}

type PublicService struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	ImageURL    string    `json:"imageUrl,omitempty"`
	URL         string    `json:"url,omitempty"`
	DeviceCount int       `json:"deviceCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Device struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Platform         string    `json:"platform"`
	Active           bool      `json:"active"`
	PushToStartToken string    `json:"pushToStartToken,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type PublicDevice struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	Platform                   string    `json:"platform"`
	Active                     bool      `json:"active"`
	PushToStartTokenRegistered bool      `json:"pushToStartTokenRegistered"`
	CreatedAt                  time.Time `json:"createdAt"`
	UpdatedAt                  time.Time `json:"updatedAt"`
}

type DeviceRegisterRequest struct {
	Name             string `json:"name"`
	Platform         string `json:"platform"`
	PushToStartToken string `json:"pushToStartToken,omitempty"`
}

type ServiceCreateRequest struct {
	Title    string `json:"title"`
	ImageURL string `json:"imageUrl,omitempty"`
	URL      string `json:"url,omitempty"`
}

type ServiceUpdateRequest struct {
	Title    *string `json:"title,omitempty"`
	ImageURL *string `json:"imageUrl,omitempty"`
	URL      *string `json:"url,omitempty"`
}

type ServiceCreateResponse struct {
	Service PublicService `json:"service"`
	Token   string        `json:"token"`
}

func (s *Service) UnmarshalJSON(data []byte) error {
	type serviceAlias Service
	var raw struct {
		serviceAlias
		Devices      json.RawMessage `json:"Devices"`
		LowerDevices json.RawMessage `json:"devices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = Service(raw.serviceAlias)
	devicePayload := raw.Devices
	if len(devicePayload) == 0 {
		devicePayload = raw.LowerDevices
	}
	if len(devicePayload) == 0 || string(devicePayload) == "null" {
		return nil
	}
	var devices []Device
	if err := json.Unmarshal(devicePayload, &devices); err == nil {
		s.Devices = devices
		return nil
	}
	var oldIDs []string
	if err := json.Unmarshal(devicePayload, &oldIDs); err != nil {
		return err
	}
	for _, id := range oldIDs {
		s.Devices = append(s.Devices, Device{ID: id, Name: id, Platform: "ios", Active: true})
	}
	return nil
}

func (s *Store) RegisterService(service Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if service.ID == "" {
		service.ID = "svc_" + randomID()
	}
	if service.CreatedAt.IsZero() {
		service.CreatedAt = time.Now().UTC()
	}
	if service.UpdatedAt.IsZero() {
		service.UpdatedAt = service.CreatedAt
	}
	for i := range service.Devices {
		normalizeDevice(&service.Devices[i])
	}
	normalizeServiceLimits(&service)
	s.services[service.Token] = service
}

func (s *Store) CreateService(req ServiceCreateRequest) (ServiceCreateResponse, error) {
	if err := validateServiceCreate(req); err != nil {
		return ServiceCreateResponse{}, err
	}
	now := time.Now().UTC()
	service := Service{
		ID:        "svc_" + randomID(),
		Token:     "beam_" + randomID(),
		Title:     strings.TrimSpace(req.Title),
		ImageURL:  req.ImageURL,
		URL:       req.URL,
		Limits:    defaultServiceLimits(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[service.Token] = service
	return ServiceCreateResponse{Service: service.Public(), Token: service.Token}, nil
}

func (s *Store) Services() []PublicService {
	s.mu.Lock()
	defer s.mu.Unlock()
	services := make([]PublicService, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service.Public())
	}
	sortServices(services)
	return services
}

func (s *Store) Service(id string) (PublicService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(id)
	if !ok {
		return PublicService{}, ErrNotFound
	}
	return service.Public(), nil
}

func (s *Store) ServiceEvents(id string) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(id)
	if !ok {
		return nil, ErrNotFound
	}
	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		if eventVisibleToService(event, service) {
			events = append(events, event)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	if len(events) > 50 {
		events = events[:50]
	}
	return events, nil
}

func (s *Store) UpdateService(id string, req ServiceUpdateRequest) (PublicService, error) {
	if err := validateServiceUpdate(req); err != nil {
		return PublicService{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(id)
	if !ok {
		return PublicService{}, ErrNotFound
	}
	if req.Title != nil {
		service.Title = strings.TrimSpace(*req.Title)
	}
	if req.ImageURL != nil {
		service.ImageURL = *req.ImageURL
	}
	if req.URL != nil {
		service.URL = *req.URL
	}
	service.UpdatedAt = time.Now().UTC()
	delete(s.services, service.Token)
	s.services[service.Token] = service
	return service.Public(), nil
}

func (s *Store) DeleteService(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(id)
	if !ok {
		return ErrNotFound
	}
	delete(s.services, service.Token)
	return nil
}

func (s *Store) RotateServiceToken(id string) (ServiceCreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(id)
	if !ok {
		return ServiceCreateResponse{}, ErrNotFound
	}
	oldToken := service.Token
	service.Token = "beam_" + randomID()
	service.RevokedTokens = append(service.RevokedTokens, oldToken)
	service.UpdatedAt = time.Now().UTC()
	delete(s.services, oldToken)
	s.services[service.Token] = service
	return ServiceCreateResponse{Service: service.Public(), Token: service.Token}, nil
}

func (s *Store) Devices(serviceID string) ([]PublicDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(serviceID)
	if !ok {
		return nil, ErrNotFound
	}
	devices := append([]Device(nil), service.Devices...)
	sortDevices(devices)
	return publicDevices(devices), nil
}

func (s *Store) RegisterDevice(serviceID string, req DeviceRegisterRequest) (PublicDevice, error) {
	if err := validateDeviceRegister(req); err != nil {
		return PublicDevice{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(serviceID)
	if !ok {
		return PublicDevice{}, ErrNotFound
	}
	now := time.Now().UTC()
	device := Device{
		ID:               "dev_" + randomID(),
		Name:             strings.TrimSpace(req.Name),
		Platform:         strings.ToLower(strings.TrimSpace(req.Platform)),
		Active:           true,
		PushToStartToken: strings.TrimSpace(req.PushToStartToken),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	service.Devices = append(service.Devices, device)
	service.UpdatedAt = now
	delete(s.services, service.Token)
	s.services[service.Token] = service
	return device.Public(), nil
}

func (s *Store) DeactivateDevice(serviceID, deviceID string) (PublicDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceByID(serviceID)
	if !ok {
		return PublicDevice{}, ErrNotFound
	}
	for i, device := range service.Devices {
		if device.ID == deviceID {
			device.Active = false
			device.UpdatedAt = time.Now().UTC()
			service.Devices[i] = device
			service.UpdatedAt = device.UpdatedAt
			delete(s.services, service.Token)
			s.services[service.Token] = service
			return device.Public(), nil
		}
	}
	return PublicDevice{}, ErrNotFound
}

func (s *Store) serviceByID(id string) (Service, bool) {
	for _, service := range s.services {
		if service.ID == id {
			return service, true
		}
	}
	return Service{}, false
}

func (s Service) Public() PublicService {
	return PublicService{
		ID:          s.ID,
		Title:       s.Title,
		ImageURL:    s.ImageURL,
		URL:         s.URL,
		DeviceCount: len(s.Devices),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (d Device) Public() PublicDevice {
	return PublicDevice{
		ID:                         d.ID,
		Name:                       d.Name,
		Platform:                   d.Platform,
		Active:                     d.Active,
		PushToStartTokenRegistered: strings.TrimSpace(d.PushToStartToken) != "",
		CreatedAt:                  d.CreatedAt,
		UpdatedAt:                  d.UpdatedAt,
	}
}

func publicDevices(devices []Device) []PublicDevice {
	public := make([]PublicDevice, 0, len(devices))
	for _, device := range devices {
		public = append(public, device.Public())
	}
	return public
}

func sortServices(services []PublicService) {
	sort.Slice(services, func(i, j int) bool {
		return services[i].CreatedAt.Before(services[j].CreatedAt)
	})
}

func activeDeviceIDs(devices []Device) map[string]bool {
	ids := map[string]bool{}
	for _, device := range devices {
		if device.Active {
			ids[device.ID] = true
		}
	}
	return ids
}

func countActiveDevices(devices []Device) int {
	count := 0
	for _, device := range devices {
		if device.Active {
			count++
		}
	}
	return count
}

func sortDevices(devices []Device) {
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})
}

func normalizeDevice(device *Device) {
	if device.ID == "" {
		device.ID = "dev_" + randomID()
	}
	if device.Name == "" {
		device.Name = device.ID
	}
	if device.Platform == "" {
		device.Platform = "ios"
	}
	if device.CreatedAt.IsZero() {
		device.CreatedAt = time.Now().UTC()
	}
	if device.UpdatedAt.IsZero() {
		device.UpdatedAt = device.CreatedAt
	}
}
