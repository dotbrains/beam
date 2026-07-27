package beam

import (
	"encoding/json"
	"net/http"
)

func decodeResponseAnswer(w http.ResponseWriter, r *http.Request) (ResponseAnswerRequest, bool) {
	body := readBody(w, r)
	if body == nil {
		return ResponseAnswerRequest{}, false
	}
	var req ResponseAnswerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "Invalid JSON"})
		return ResponseAnswerRequest{}, false
	}
	return req, true
}
