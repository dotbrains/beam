package beam

import "time"

type ProviderDiagnostic struct {
	Provider  string    `json:"provider"`
	Operation string    `json:"operation"`
	DeviceID  string    `json:"deviceId,omitempty"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func notificationDiagnostics(devices []Device, requestedIDs []string, now time.Time) []ProviderDiagnostic {
	targets := activityTargetDeviceIDs(devices, requestedIDs)
	return deliveryDiagnostics("notification", targets, len(targets) == 0, now)
}

func activityDiagnostics(operation string, targetIDs []string, now time.Time) []ProviderDiagnostic {
	return deliveryDiagnostics(operation, targetIDs, len(targetIDs) == 0, now)
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
