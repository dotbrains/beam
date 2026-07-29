package beam

import "time"

type Snapshot struct {
	Accounts    map[string]Account           `json:"accounts,omitempty"`
	Services    map[string]Service           `json:"services"`
	Events      map[string]Event             `json:"events"`
	Activities  map[string]Activity          `json:"activities"`
	AuthDevices map[string]AuthDevice        `json:"authDevices"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

func NewStore() *Store {
	return NewStoreWithProvider(LocalPushProvider{})
}

func NewStoreWithProvider(provider PushProvider) *Store {
	if provider == nil {
		provider = LocalPushProvider{}
	}
	store := &Store{
		accounts:    map[string]Account{},
		services:    map[string]Service{},
		events:      map[string]Event{},
		activities:  map[string]Activity{},
		authDevices: map[string]AuthDevice{},
		idempotency: map[string]IdempotencyRecord{},
		provider:    provider,
	}
	now := time.Now().UTC()
	store.RegisterService(Service{
		ID:        "svc_dev",
		Token:     "dev_token",
		Title:     "Beam",
		Devices:   []Device{{ID: "dev_local", Name: "Local Device", Platform: "ios", Active: true, CreatedAt: now, UpdatedAt: now}},
		CreatedAt: now,
		UpdatedAt: now,
	})
	return store
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Accounts:    copyMap(s.accounts),
		Services:    copyMap(s.services),
		Events:      copyMap(s.events),
		Activities:  copyMap(s.activities),
		AuthDevices: copyMap(s.authDevices),
		Idempotency: copyMap(s.idempotency),
	}
}

func NewStoreFromSnapshot(snapshot Snapshot) *Store {
	return NewStoreFromSnapshotWithProvider(snapshot, LocalPushProvider{})
}

func NewStoreFromSnapshotWithProvider(snapshot Snapshot, provider PushProvider) *Store {
	if provider == nil {
		provider = LocalPushProvider{}
	}
	services := copyMap(snapshot.Services)
	accounts := copyMap(snapshot.Accounts)
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
		normalizeAccountForService(accounts, service)
		normalized[service.TokenHash] = service
	}
	store := &Store{
		accounts:    accounts,
		services:    normalized,
		events:      copyMap(snapshot.Events),
		activities:  normalizeActivities(snapshot.Activities, normalized),
		authDevices: copyMap(snapshot.AuthDevices),
		idempotency: copyMap(snapshot.Idempotency),
		provider:    provider,
	}
	if store.authDevices == nil {
		store.authDevices = map[string]AuthDevice{}
	}
	if store.accounts == nil {
		store.accounts = map[string]Account{}
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

func normalizeActivities(activities map[string]Activity, services map[string]Service) map[string]Activity {
	normalized := map[string]Activity{}
	defaultServiceID := ""
	if len(services) == 1 {
		for _, service := range services {
			defaultServiceID = service.ID
		}
	}
	for _, activity := range activities {
		if activity.ID == "" {
			continue
		}
		if activity.ServiceID == "" {
			activity.ServiceID = defaultServiceID
		}
		normalized[activity.ID] = activity
		if activity.ServiceID != "" && activity.Key != "" {
			normalized[activityKey(activity.ServiceID, activity.Key)] = activity
		}
	}
	return normalized
}

func copyMap[V any](src map[string]V) map[string]V {
	dst := make(map[string]V, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
