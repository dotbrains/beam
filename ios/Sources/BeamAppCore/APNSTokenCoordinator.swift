import Foundation

public protocol BeamDeviceRegistering: Sendable {
    func register(_ registration: BeamDeviceRegistration) async throws -> BeamDevice
}

extension BeamDeviceRegistrar: BeamDeviceRegistering {}

public struct BeamAPNSTokenCoordinator<Registrar: BeamDeviceRegistering>: Sendable {
    public let deviceName: String
    public let registrar: Registrar

    public init(deviceName: String, registrar: Registrar) {
        self.deviceName = deviceName
        self.registrar = registrar
    }

    public func registerDeviceToken(
        _ deviceToken: Data,
        pushToStartToken: String? = nil
    ) async throws -> BeamDevice {
        try await registrar.register(BeamDeviceRegistration(
            name: deviceName,
            pushToken: Self.hexToken(deviceToken),
            pushToStartToken: pushToStartToken
        ))
    }

    public static func hexToken(_ data: Data) -> String {
        data.map { String(format: "%02x", $0) }.joined()
    }
}
