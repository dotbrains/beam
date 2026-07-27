package beam

type Backend interface {
	SendNotification(token string, req NotificationRequest, idemKey, fingerprint string) (Event, bool, error)
	Event(token, id string) (Event, error)
	CancelEvent(token, id string) (Event, error)
	StartActivity(token string, req ActivityRequest, idemKey, fingerprint string) (Activity, bool, error)
	Activity(token, id string) (Activity, error)
	UpdateActivity(token, id string, req ActivityRequest) (Activity, error)
	EndActivity(token, id string, req ActivityRequest) (Activity, error)
}
