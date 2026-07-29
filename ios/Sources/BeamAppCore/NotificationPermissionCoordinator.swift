import Foundation

public struct BeamNotificationAuthorizationOptions: OptionSet, Sendable {
    public let rawValue: Int

    public init(rawValue: Int) {
        self.rawValue = rawValue
    }

    public static let alert = BeamNotificationAuthorizationOptions(rawValue: 1 << 0)
    public static let sound = BeamNotificationAuthorizationOptions(rawValue: 1 << 1)
    public static let badge = BeamNotificationAuthorizationOptions(rawValue: 1 << 2)

    public static let standard: BeamNotificationAuthorizationOptions = [.alert, .sound, .badge]
}

public protocol BeamNotificationAuthorizing: Sendable {
    func requestAuthorization(options: BeamNotificationAuthorizationOptions) async throws -> BeamNotificationAuthorization
}

public protocol BeamRemoteNotificationRegistering: Sendable {
    func registerForRemoteNotifications() async throws
}

public struct BeamNotificationPermissionResult: Equatable, Sendable {
    public let state: BeamAppDeviceState
    public let shouldRegisterForRemoteNotifications: Bool

    public init(
        state: BeamAppDeviceState,
        shouldRegisterForRemoteNotifications: Bool
    ) {
        self.state = state
        self.shouldRegisterForRemoteNotifications = shouldRegisterForRemoteNotifications
    }
}

public struct BeamNotificationPermissionCoordinator<
    Authorizer: BeamNotificationAuthorizing,
    RemoteRegistrar: BeamRemoteNotificationRegistering
>: Sendable {
    public let authorizer: Authorizer
    public let remoteRegistrar: RemoteRegistrar
    public let options: BeamNotificationAuthorizationOptions

    public init(
        authorizer: Authorizer,
        remoteRegistrar: RemoteRegistrar,
        options: BeamNotificationAuthorizationOptions = .standard
    ) {
        self.authorizer = authorizer
        self.remoteRegistrar = remoteRegistrar
        self.options = options
    }

    public func requestNotifications(from state: BeamAppDeviceState) async throws -> BeamNotificationPermissionResult {
        let authorization = try await authorizer.requestAuthorization(options: options)
        let updatedState = state.applying(notificationAuthorization: authorization)
        let shouldRegister = authorization.allowsNotificationDelivery
        if shouldRegister {
            try await remoteRegistrar.registerForRemoteNotifications()
        }
        return BeamNotificationPermissionResult(
            state: updatedState,
            shouldRegisterForRemoteNotifications: shouldRegister
        )
    }
}
