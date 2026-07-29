package beam

import (
	"encoding/json"
	"time"
)

type Snapshot struct {
	Accounts    map[string]Account           `json:"accounts,omitempty"`
	Services    map[string]Service           `json:"services"`
	Events      map[string]Event             `json:"events"`
	Activities  map[string]Activity          `json:"activities"`
	AuthDevices map[string]AuthDevice        `json:"authDevices"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

type snapshotJSON struct {
	Accounts    map[string]Account           `json:"accounts,omitempty"`
	Services    map[string]Service           `json:"services"`
	Events      map[string]eventSnapshot     `json:"events"`
	Activities  map[string]Activity          `json:"activities"`
	AuthDevices map[string]AuthDevice        `json:"authDevices"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

type eventAlias Event

type eventSnapshot struct {
	eventAlias
	Response *responseSnapshot `json:"response,omitempty"`
}

type responseAlias ResponseState

type responseSnapshot struct {
	responseAlias
	CallbackURL   string `json:"callbackUrl,omitempty"`
	CallbackToken string `json:"callbackToken,omitempty"`
}

func (s Snapshot) MarshalJSON() ([]byte, error) {
	events := make(map[string]eventSnapshot, len(s.Events))
	for key, event := range s.Events {
		snapshotEvent := eventSnapshot{eventAlias: eventAlias(event)}
		if event.Response != nil {
			response := responseSnapshot{
				responseAlias: responseAlias(*event.Response),
				CallbackURL:   event.Response.CallbackURL,
				CallbackToken: event.Response.CallbackToken,
			}
			snapshotEvent.eventAlias.Response = nil
			snapshotEvent.Response = &response
		}
		events[key] = snapshotEvent
	}
	return json.Marshal(snapshotJSON{
		Accounts:    s.Accounts,
		Services:    s.Services,
		Events:      events,
		Activities:  s.Activities,
		AuthDevices: s.AuthDevices,
		Idempotency: s.Idempotency,
	})
}

func (s *Snapshot) UnmarshalJSON(data []byte) error {
	var payload snapshotJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	events := make(map[string]Event, len(payload.Events))
	for key, snapshotEvent := range payload.Events {
		event := Event(snapshotEvent.eventAlias)
		if snapshotEvent.Response != nil {
			response := ResponseState(snapshotEvent.Response.responseAlias)
			response.CallbackURL = snapshotEvent.Response.CallbackURL
			response.CallbackToken = snapshotEvent.Response.CallbackToken
			event.Response = &response
		}
		events[key] = event
	}
	*s = Snapshot{
		Accounts:    payload.Accounts,
		Services:    payload.Services,
		Events:      events,
		Activities:  payload.Activities,
		AuthDevices: payload.AuthDevices,
		Idempotency: payload.Idempotency,
	}
	return nil
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
		authDevices: normalizeAuthDevices(snapshot.AuthDevices),
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

func normalizeAuthDevices(devices map[string]AuthDevice) map[string]AuthDevice {
	normalized := map[string]AuthDevice{}
	for key, device := range devices {
		if device.DeviceCode == "" {
			device.DeviceCode = key
		}
		if device.TokenHash == "" && device.Token != "" {
			device.TokenHash = hashToken(device.Token)
		}
		device.Token = ""
		normalized[device.DeviceCode] = device
	}
	return normalized
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
