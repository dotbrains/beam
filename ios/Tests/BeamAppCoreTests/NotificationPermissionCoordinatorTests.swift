import Testing

@testable import BeamAppCore

@Test func permissionCoordinatorRegistersWhenAuthorizationAllowsDelivery() async throws {
    let authorizer = MockNotificationAuthorizer(authorization: .authorized)
    let registrar = MockRemoteNotificationRegistrar()
    let coordinator = BeamNotificationPermissionCoordinator(
        authorizer: authorizer,
        remoteRegistrar: registrar
    )

    let result = try await coordinator.requestNotifications(from: BeamAppDeviceState())

    #expect(result.state.notificationAuthorization == .authorized)
    #expect(result.shouldRegisterForRemoteNotifications)
    #expect(await registrar.registrationCount() == 1)
    #expect(await authorizer.lastOptions() == .standard)
}

@Test func permissionCoordinatorDoesNotRegisterWhenAuthorizationIsDenied() async throws {
    let authorizer = MockNotificationAuthorizer(authorization: .denied)
    let registrar = MockRemoteNotificationRegistrar()
    let coordinator = BeamNotificationPermissionCoordinator(
        authorizer: authorizer,
        remoteRegistrar: registrar
    )

    let result = try await coordinator.requestNotifications(from: BeamAppDeviceState())

    #expect(result.state.notificationAuthorization == .denied)
    #expect(!result.shouldRegisterForRemoteNotifications)
    #expect(await registrar.registrationCount() == 0)
}

@Test func permissionCoordinatorPreservesExistingDeviceState() async throws {
    let authorizer = MockNotificationAuthorizer(authorization: .provisional)
    let registrar = MockRemoteNotificationRegistrar()
    let coordinator = BeamNotificationPermissionCoordinator(
        authorizer: authorizer,
        remoteRegistrar: registrar,
        options: [.alert, .badge]
    )
    let initial = BeamAppDeviceState(device: BeamDevice(
        id: "dev_ios",
        name: "Nick's iPhone",
        platform: "ios",
        active: true,
        pushTokenRegistered: true,
        pushToStartTokenRegistered: false
    ))

    let result = try await coordinator.requestNotifications(from: initial)

    #expect(result.state.device?.id == "dev_ios")
    #expect(result.state.notificationAuthorization == .provisional)
    #expect(result.state.canReceiveNotifications)
    #expect(!result.state.canStartLiveActivities)
    #expect(await registrar.registrationCount() == 1)
    #expect(await authorizer.lastOptions() == [.alert, .badge])
}

private actor MockNotificationAuthorizer: BeamNotificationAuthorizing {
    private let authorization: BeamNotificationAuthorization
    private var capturedOptions: BeamNotificationAuthorizationOptions?

    init(authorization: BeamNotificationAuthorization) {
        self.authorization = authorization
    }

    func requestAuthorization(options: BeamNotificationAuthorizationOptions) async throws -> BeamNotificationAuthorization {
        capturedOptions = options
        return authorization
    }

    func lastOptions() -> BeamNotificationAuthorizationOptions? {
        capturedOptions
    }
}

private actor MockRemoteNotificationRegistrar: BeamRemoteNotificationRegistering {
    private var count = 0

    func registerForRemoteNotifications() async throws {
        count += 1
    }

    func registrationCount() -> Int {
        count
    }
}
