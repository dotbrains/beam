import Foundation
import Testing

@testable import BeamAppCore

@Test func hexEncodesAPNSDeviceTokenBytes() {
    let token = BeamAPNSTokenCoordinator<MockDeviceRegistrar>.hexToken(Data([0x00, 0x0f, 0xa4, 0xff]))

    #expect(token == "000fa4ff")
}

@Test func coordinatorRegistersNotificationAndPushToStartTokens() async throws {
    let registrar = MockDeviceRegistrar(device: BeamDevice(
        id: "dev_ios",
        name: "Nick's iPhone",
        platform: "ios",
        active: true,
        pushTokenRegistered: true,
        pushToStartTokenRegistered: true
    ))
    let coordinator = BeamAPNSTokenCoordinator(deviceName: "Nick's iPhone", registrar: registrar)

    let device = try await coordinator.registerDeviceToken(
        Data([0xde, 0xad, 0xbe, 0xef]),
        pushToStartToken: "activity_secret_123"
    )
    let registration = await registrar.lastRegistration()

    #expect(device.id == "dev_ios")
    #expect(registration?.name == "Nick's iPhone")
    #expect(registration?.platform == "ios")
    #expect(registration?.pushToken == "deadbeef")
    #expect(registration?.pushToStartToken == "activity_secret_123")
}

private actor MockDeviceRegistrar: BeamDeviceRegistering {
    private let device: BeamDevice
    private var capturedRegistration: BeamDeviceRegistration?

    init(device: BeamDevice) {
        self.device = device
    }

    func register(_ registration: BeamDeviceRegistration) async throws -> BeamDevice {
        capturedRegistration = registration
        return device
    }

    func lastRegistration() -> BeamDeviceRegistration? {
        capturedRegistration
    }
}
