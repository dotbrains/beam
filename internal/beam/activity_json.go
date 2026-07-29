package beam

import "encoding/json"

func (req ActivityRequest) MarshalJSON() ([]byte, error) {
	type activityRequest ActivityRequest
	payload, err := json.Marshal(activityRequest(req))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	if req.DetailSet && req.Detail == nil {
		fields["detail"] = []byte("null")
	}
	if req.ProgressSet && req.Progress == nil {
		fields["progress"] = []byte("null")
	}
	return json.Marshal(fields)
}
