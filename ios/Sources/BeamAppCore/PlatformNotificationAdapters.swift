import Foundation

#if canImport(UserNotifications)
@preconcurrency import UserNotifications

public final class BeamUserNotificationAuthorizer: BeamNotificationAuthorizing, @unchecked Sendable {
    public let center: UNUserNotificationCenter

    public init(center: UNUserNotificationCenter = .current()) {
        self.center = center
    }

    public func requestAuthorization(options: BeamNotificationAuthorizationOptions) async throws -> BeamNotificationAuthorization {
        let granted = try await center.requestAuthorization(options: options.userNotificationOptions)
        if granted {
            return .authorized
        }
        let settings = await center.notificationSettings()
        return Self.authorization(from: settings.authorizationStatus)
    }

    public static func authorization(from status: UNAuthorizationStatus) -> BeamNotificationAuthorization {
        switch status {
        case .notDetermined:
            .notDetermined
        case .denied:
            .denied
        case .authorized:
            .authorized
        case .provisional:
            .provisional
        case .ephemeral:
            .ephemeral
        @unknown default:
            .denied
        }
    }
}

public extension BeamNotificationAuthorizationOptions {
    var userNotificationOptions: UNAuthorizationOptions {
        var options = UNAuthorizationOptions()
        if contains(.alert) {
            options.insert(.alert)
        }
        if contains(.sound) {
            options.insert(.sound)
        }
        if contains(.badge) {
            options.insert(.badge)
        }
        return options
    }
}
#endif

#if os(iOS) && canImport(UIKit)
import UIKit

@MainActor
public struct BeamUIApplicationRemoteNotificationRegistrar: BeamRemoteNotificationRegistering {
    public let application: UIApplication

    public init(application: UIApplication = .shared) {
        self.application = application
    }

    public func registerForRemoteNotifications() async throws {
        application.registerForRemoteNotifications()
    }
}
#endif
