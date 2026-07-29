import Testing

@testable import BeamAppCore

@Test func appHomeViewStateShowsReadyDevice() {
    let state = BeamAppHomeViewState(
        serviceName: "Beam",
        deviceName: "Fallback",
        deviceState: BeamAppDeviceState(
            device: homeDevice(),
            notificationAuthorization: .authorized
        )
    )

    #expect(state.title == "Beam")
    #expect(state.deviceName == "Nick's iPhone")
    #expect(state.statusTitle == "Ready")
    #expect(state.statusDetail == "This device can receive Beam pushes.")
    #expect(state.notificationLine == "Notifications ready")
    #expect(state.liveActivityLine == "Live Activities ready")
    #expect(state.permissionLine == "Permission authorized")
}

@Test func appHomeViewStateShowsUnregisteredDevice() {
    let state = BeamAppHomeViewState(
        serviceName: "Beam",
        deviceName: "Nick's iPhone",
        deviceState: BeamAppDeviceState()
    )

    #expect(state.deviceName == "Nick's iPhone")
    #expect(state.statusTitle == "Device not registered")
    #expect(state.statusDetail == "Connect this iPhone to a Beam service.")
    #expect(state.notificationLine == "Notifications unavailable")
    #expect(state.liveActivityLine == "Live Activities unavailable")
    #expect(state.permissionLine == "Permission not requested")
}

@Test func appHomeViewStateShowsSpecificRegistrationProblems() {
    let cases: [(BeamDeviceRegistrationStatus, BeamAppDeviceState, String)] = [
        (.inactive, BeamAppDeviceState(device: homeDevice(active: false), notificationAuthorization: .authorized), "Device inactive"),
        (.notificationTokenMissing, BeamAppDeviceState(device: homeDevice(pushTokenRegistered: false), notificationAuthorization: .authorized), "Notification token needed"),
        (.liveActivityTokenMissing, BeamAppDeviceState(device: homeDevice(pushToStartTokenRegistered: false), notificationAuthorization: .authorized), "Live Activity token needed")
    ]

    for (status, deviceState, title) in cases {
        let state = BeamAppHomeViewState(
            serviceName: "Beam",
            deviceName: "Nick's iPhone",
            deviceState: deviceState
        )

        #expect(state.registrationStatus == status)
        #expect(state.statusTitle == title)
    }
}

private func homeDevice(
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
