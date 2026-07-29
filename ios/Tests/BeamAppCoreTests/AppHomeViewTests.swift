import SwiftUI
import Testing

@testable import BeamAppCore

@MainActor
@Test func appHomeViewCompilesWithReadyState() {
    let view = BeamAppHomeView(state: BeamAppHomeViewState(
        serviceName: "Beam",
        deviceName: "Nick's iPhone",
        deviceState: BeamAppDeviceState(
            device: BeamDevice(
                id: "dev_ios",
                name: "Nick's iPhone",
                platform: "ios",
                active: true,
                pushTokenRegistered: true,
                pushToStartTokenRegistered: true
            ),
            notificationAuthorization: .authorized
        )
    ))

    assertHomeViewType(view)
}

private func assertHomeViewType<V: View>(_ view: V) {
    _ = V.self
}
