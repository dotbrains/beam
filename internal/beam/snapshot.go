package beam

import "time"

type Snapshot struct {
	Services    map[string]Service           `json:"services"`
	Events      map[string]Event             `json:"events"`
	Activities  map[string]Activity          `json:"activities"`
	AuthDevices map[string]AuthDevice        `json:"authDevices"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Services:    copyMap(s.services),
		Events:      copyMap(s.events),
		Activities:  copyMap(s.activities),
		AuthDevices: copyMap(s.authDevices),
		Idempotency: copyMap(s.idempotency),
	}
}

func NewStoreFromSnapshot(snapshot Snapshot) *Store {
	services := copyMap(snapshot.Services)
	normalized := map[string]Service{}
	for key, service := range services {
		if service.ID == "" {
			service.ID = "svc_" + randomID()
		}
		if service.CreatedAt.IsZero() {
			service.CreatedAt = service.UpdatedAt
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
		normalizeServiceToken(&service, key)
		normalizeServiceLimits(&service)
		normalized[service.TokenHash] = service
	}
	store := &Store{
		services:    normalized,
		events:      copyMap(snapshot.Events),
		activities:  copyMap(snapshot.Activities),
		authDevices: copyMap(snapshot.AuthDevices),
		idempotency: copyMap(snapshot.Idempotency),
	}
	if store.authDevices == nil {
		store.authDevices = map[string]AuthDevice{}
	}
	if len(store.services) == 0 {
		now := time.Now().UTC()
		store.RegisterService(Service{
			ID:        "svc_dev",
			Token:     "dev_token",
			Title:     "Beam",
			Devices:   []Device{{ID: "dev_local", Name: "Local Device", Platform: "ios", Active: true, CreatedAt: now, UpdatedAt: now}},
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return store
}

func copyMap[V any](src map[string]V) map[string]V {
	dst := make(map[string]V, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
