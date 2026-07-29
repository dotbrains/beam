import Foundation

public enum BeamNotificationAuthorization: String, Equatable, Sendable {
    case notDetermined
    case denied
    case authorized
    case provisional
    case ephemeral

    public var allowsNotificationDelivery: Bool {
        switch self {
        case .authorized, .provisional, .ephemeral:
            true
        case .notDetermined, .denied:
            false
        }
    }
}

public enum BeamDeviceRegistrationStatus: Equatable, Sendable {
    case unregistered
    case notificationTokenMissing
    case liveActivityTokenMissing
    case inactive
    case ready
}

public struct BeamAppDeviceState: Equatable, Sendable {
    public let device: BeamDevice?
    public let notificationAuthorization: BeamNotificationAuthorization

    public init(
        device: BeamDevice? = nil,
        notificationAuthorization: BeamNotificationAuthorization = .notDetermined
    ) {
        self.device = device
        self.notificationAuthorization = notificationAuthorization
    }

    public var registrationStatus: BeamDeviceRegistrationStatus {
        guard let device else {
            return .unregistered
        }
        guard device.active else {
            return .inactive
        }
        guard device.pushTokenRegistered else {
            return .notificationTokenMissing
        }
        guard device.pushToStartTokenRegistered else {
            return .liveActivityTokenMissing
        }
        return .ready
    }

    public var canReceiveNotifications: Bool {
        guard let device else {
            return false
        }
        return device.active &&
            device.platform == "ios" &&
            device.pushTokenRegistered &&
            notificationAuthorization.allowsNotificationDelivery
    }

    public var canStartLiveActivities: Bool {
        guard let device else {
            return false
        }
        return device.active &&
            device.platform == "ios" &&
            device.pushToStartTokenRegistered
    }

    public func applying(device: BeamDevice) -> BeamAppDeviceState {
        BeamAppDeviceState(
            device: device,
            notificationAuthorization: notificationAuthorization
        )
    }

    public func applying(notificationAuthorization: BeamNotificationAuthorization) -> BeamAppDeviceState {
        BeamAppDeviceState(
            device: device,
            notificationAuthorization: notificationAuthorization
        )
    }
}
