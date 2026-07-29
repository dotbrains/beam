import Testing

@testable import BeamAppCore

@Test func unregisteredAppDeviceStateIsNotReady() {
    let state = BeamAppDeviceState(notificationAuthorization: .authorized)

    #expect(state.registrationStatus == .unregistered)
    #expect(!state.canReceiveNotifications)
    #expect(!state.canStartLiveActivities)
}

@Test func activeRegisteredIOSDeviceCanReceiveNotifications() {
    let state = BeamAppDeviceState(
        device: device(),
        notificationAuthorization: .authorized
    )

    #expect(state.registrationStatus == .ready)
    #expect(state.canReceiveNotifications)
    #expect(state.canStartLiveActivities)
}

@Test func deniedNotificationAuthorizationBlocksNotificationDeliveryOnly() {
    let state = BeamAppDeviceState(
        device: device(),
        notificationAuthorization: .denied
    )

    #expect(state.registrationStatus == .ready)
    #expect(!state.canReceiveNotifications)
    #expect(state.canStartLiveActivities)
}

@Test func inactiveDeviceBlocksNotificationsAndLiveActivities() {
    let state = BeamAppDeviceState(
        device: device(active: false),
        notificationAuthorization: .authorized
    )

    #expect(state.registrationStatus == .inactive)
    #expect(!state.canReceiveNotifications)
    #expect(!state.canStartLiveActivities)
}

@Test func missingTokensReportSpecificRegistrationStatus() {
    let notificationState = BeamAppDeviceState(
        device: device(pushTokenRegistered: false),
        notificationAuthorization: .authorized
    )
    let activityState = BeamAppDeviceState(
        device: device(pushToStartTokenRegistered: false),
        notificationAuthorization: .authorized
    )

    #expect(notificationState.registrationStatus == .notificationTokenMissing)
    #expect(activityState.registrationStatus == .liveActivityTokenMissing)
    #expect(!notificationState.canReceiveNotifications)
    #expect(!activityState.canStartLiveActivities)
}

@Test func appDeviceStateAppliesDeviceAndAuthorizationChanges() {
    let initial = BeamAppDeviceState()
    let registered = initial.applying(device: device()).applying(notificationAuthorization: .provisional)

    #expect(registered.device?.id == "dev_ios")
    #expect(registered.notificationAuthorization == .provisional)
    #expect(registered.canReceiveNotifications)
}

private func device(
    active: Bool = true,
    pushTokenRegistered: Bool = true,
    pushToStartTokenRegistered: Bool = true
) -> BeamDevice {
    BeamDevice(
        id: "dev_ios",
        name: "Nick's iPhone",
        platform: "ios",
        active: active,
        pushTokenRegistered: pushTokenRegistered,
        pushToStartTokenRegistered: pushToStartTokenRegistered
    )
}
