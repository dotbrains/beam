package beam

import (
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
	Devices       []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevokedTokens []string
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

func sortServices(services []PublicService) {
	sort.Slice(services, func(i, j int) bool {
		return services[i].CreatedAt.Before(services[j].CreatedAt)
	})
}
