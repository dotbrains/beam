package beam

import (
	"testing"
	"time"
)

func TestSnapshotNormalizesLegacyAuthDeviceToken(t *testing.T) {
	now := time.Now().UTC()
	store := NewStoreFromSnapshot(Snapshot{
		Services: map[string]Service{},
		Events:   map[string]Event{},
		AuthDevices: map[string]AuthDevice{
			"adc_legacy": {
				DeviceCode: "adc_legacy",
				ClientName: "Legacy",
				Token:      "beam_agent_legacy",
				Status:     "approved",
				ExpiresAt:  now.Add(time.Hour),
				CreatedAt:  now,
			},
		},
	})

	device, err := store.AuthDeviceForToken("beam_agent_legacy")
	if err != nil {
		t.Fatal(err)
	}
	if device.Token != "" || device.TokenHash == "" {
		t.Fatalf("legacy token was not normalized: %#v", device)
	}
	snapshot := store.Snapshot()
	if snapshot.AuthDevices["adc_legacy"].Token != "" || snapshot.AuthDevices["adc_legacy"].TokenHash == "" {
		t.Fatalf("snapshot contains unnormalized auth device: %#v", snapshot.AuthDevices["adc_legacy"])
	}
}
