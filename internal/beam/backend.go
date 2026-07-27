package beam

type Backend interface {
	CreateService(req ServiceCreateRequest) (ServiceCreateResponse, error)
	Services() []PublicService
	Service(id string) (PublicService, error)
	ServiceEvents(id string) ([]Event, error)
	UpdateService(id string, req ServiceUpdateRequest) (PublicService, error)
	DeleteService(id string) error
	RotateServiceToken(id string) (ServiceCreateResponse, error)
	Devices(serviceID string) ([]PublicDevice, error)
	RegisterDevice(serviceID string, req DeviceRegisterRequest) (PublicDevice, error)
	DeactivateDevice(serviceID, deviceID string) (PublicDevice, error)
	SendNotification(token string, req NotificationRequest, idemKey, fingerprint string) (Event, bool, error)
	StartAuthDevice(req AuthDeviceRequest, verifyBaseURL string) (AuthDevice, error)
	AuthDevices() []PublicAuthDevice
	ApproveAuthDevice(userCode string) (AuthDevice, error)
	AuthDeviceToken(deviceCode string) (AuthDevice, error)
	RevokeAuthDevice(deviceCode string) (AuthDevice, error)
	RevokeAuthToken(token string) (AuthDevice, error)
	Event(token, id string) (Event, error)
	CancelEvent(token, id string) (Event, error)
	RespondEvent(token, id string, req ResponseAnswerRequest) (Event, error)
	StartActivity(token string, req ActivityRequest, idemKey, fingerprint string) (Activity, bool, error)
	Activities(token string) ([]Activity, error)
	Activity(token, id string) (Activity, error)
	UpdateActivity(token, id string, req ActivityRequest) (Activity, error)
	EndActivity(token, id string, req ActivityRequest) (Activity, error)
}
