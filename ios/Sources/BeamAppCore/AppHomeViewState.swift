import Foundation

public struct BeamAppHomeViewState: Equatable, Sendable {
    public let serviceName: String
    public let deviceName: String
    public let registrationStatus: BeamDeviceRegistrationStatus
    public let notificationAuthorization: BeamNotificationAuthorization
    public let canReceiveNotifications: Bool
    public let canStartLiveActivities: Bool

    public init(
        serviceName: String,
        deviceName: String,
        deviceState: BeamAppDeviceState
    ) {
        self.serviceName = serviceName
        self.deviceName = deviceState.device?.name ?? deviceName
        self.registrationStatus = deviceState.registrationStatus
        self.notificationAuthorization = deviceState.notificationAuthorization
        self.canReceiveNotifications = deviceState.canReceiveNotifications
        self.canStartLiveActivities = deviceState.canStartLiveActivities
    }

    public var title: String {
        serviceName
    }

    public var statusTitle: String {
        switch registrationStatus {
        case .ready:
            "Ready"
        case .inactive:
            "Device inactive"
        case .notificationTokenMissing:
            "Notification token needed"
        case .liveActivityTokenMissing:
            "Live Activity token needed"
        case .unregistered:
            "Device not registered"
        }
    }

    public var statusDetail: String {
        switch registrationStatus {
        case .ready:
            "This device can receive Beam pushes."
        case .inactive:
            "Reactivate this device from Beam before routing pushes to it."
        case .notificationTokenMissing:
            "Allow notifications so Beam can register an APNs token."
        case .liveActivityTokenMissing:
            "Open the app again after Live Activities are enabled."
        case .unregistered:
            "Connect this iPhone to a Beam service."
        }
    }

    public var notificationLine: String {
        canReceiveNotifications ? "Notifications ready" : "Notifications unavailable"
    }

    public var liveActivityLine: String {
        canStartLiveActivities ? "Live Activities ready" : "Live Activities unavailable"
    }

    public var permissionLine: String {
        switch notificationAuthorization {
        case .authorized:
            "Permission authorized"
        case .provisional:
            "Permission provisional"
        case .ephemeral:
            "Permission ephemeral"
        case .denied:
            "Permission denied"
        case .notDetermined:
            "Permission not requested"
        }
    }
}
