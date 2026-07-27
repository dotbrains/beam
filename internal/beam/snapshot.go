package beam

type Snapshot struct {
	Services    map[string]Service           `json:"services"`
	Events      map[string]Event             `json:"events"`
	Activities  map[string]Activity          `json:"activities"`
	Idempotency map[string]IdempotencyRecord `json:"idempotency"`
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Services:    copyMap(s.services),
		Events:      copyMap(s.events),
		Activities:  copyMap(s.activities),
		Idempotency: copyMap(s.idempotency),
	}
}

func NewStoreFromSnapshot(snapshot Snapshot) *Store {
	store := &Store{
		services:    copyMap(snapshot.Services),
		events:      copyMap(snapshot.Events),
		activities:  copyMap(snapshot.Activities),
		idempotency: copyMap(snapshot.Idempotency),
	}
	if len(store.services) == 0 {
		store.RegisterService(Service{Token: "dev_token", Title: "Beam", Devices: []string{"dev_local"}})
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
