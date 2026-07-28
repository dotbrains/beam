package beam

import "time"

type PushProvider interface {
	SendNotification(req PushNotification) ([]ProviderDiagnostic, error)
	StartActivity(req ActivityPush) ([]ProviderDiagnostic, error)
	UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error)
	EndActivity(req ActivityPush) ([]ProviderDiagnostic, error)
}

type PushNotification struct {
	EventID   string
	DeviceIDs []string
	CreatedAt time.Time
}

type ActivityPush struct {
	ActivityID string
	DeviceIDs  []string
	CreatedAt  time.Time
}

type ProviderDiagnostic struct {
	Provider  string    `json:"provider"`
	Operation string    `json:"operation"`
	DeviceID  string    `json:"deviceId,omitempty"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type LocalPushProvider struct{}

func (LocalPushProvider) SendNotification(req PushNotification) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("notification", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) StartActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_start", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) UpdateActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_update", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func (LocalPushProvider) EndActivity(req ActivityPush) ([]ProviderDiagnostic, error) {
	return deliveryDiagnostics("activity_end", req.DeviceIDs, len(req.DeviceIDs) == 0, req.CreatedAt), nil
}

func acceptedDeliveryCount(diagnostics []ProviderDiagnostic) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Status == "accepted" {
			count++
		}
	}
	return count
}

func deliveryDiagnostics(operation string, targetIDs []string, noTarget bool, now time.Time) []ProviderDiagnostic {
	if noTarget {
		return []ProviderDiagnostic{{
			Provider:  "local",
			Operation: operation,
			Status:    "skipped",
			Reason:    "no_active_device",
			CreatedAt: now,
		}}
	}
	diagnostics := make([]ProviderDiagnostic, 0, len(targetIDs))
	for _, id := range targetIDs {
		diagnostics = append(diagnostics, ProviderDiagnostic{
			Provider:  "local",
			Operation: operation,
			DeviceID:  id,
			Status:    "accepted",
			CreatedAt: now,
		})
	}
	return diagnostics
}

func providerFailureDiagnostic(operation string, now time.Time) ProviderDiagnostic {
	return ProviderDiagnostic{
		Provider:  "unknown",
		Operation: operation,
		Status:    "failed",
		Reason:    "provider_failure",
		CreatedAt: now,
	}
}
