package beam

import "net/http"

func activityResponse(activity Activity) map[string]any {
	failed := 0
	for _, diagnostic := range activity.ProviderDiagnostics {
		if diagnostic.Status == "failed" || diagnostic.Status == "skipped" {
			failed++
		}
	}
	return map[string]any{
		"ok":                  true,
		"activityId":          activity.ID,
		"key":                 activity.Key,
		"deviceIds":           activity.DeviceIDs,
		"sequence":            activity.Sequence,
		"status":              activity.Status,
		"accepted":            activity.Delivered,
		"failed":              failed,
		"state":               activity.State,
		"providerDiagnostics": activity.ProviderDiagnostics,
		"expiresAt":           activity.ExpiresAt,
		"staleAt":             activity.StaleAt,
		"endedAt":             activity.EndedAt,
	}
}

func writeActivityConflict(w http.ResponseWriter, err error, activity Activity) {
	resp := activityResponse(activity)
	resp["ok"] = false
	resp["error"] = err.Error()
	writeJSON(w, http.StatusConflict, resp)
}
