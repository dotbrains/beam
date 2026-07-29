import Foundation

public struct BeamDeviceRegistration: Encodable, Equatable, Sendable {
    public let name: String
    public let platform: String
    public let pushToken: String?
    public let pushToStartToken: String?

    public init(name: String, pushToken: String? = nil, pushToStartToken: String? = nil) {
        self.name = name
        self.platform = "ios"
        self.pushToken = pushToken
        self.pushToStartToken = pushToStartToken
    }
}

public struct BeamDeviceRegistrationResponse: Decodable, Equatable, Sendable {
    public let ok: Bool
    public let device: BeamDevice
}

public struct BeamDevice: Decodable, Equatable, Sendable {
    public let id: String
    public let name: String
    public let platform: String
    public let active: Bool
    public let pushTokenRegistered: Bool
    public let pushToStartTokenRegistered: Bool
}

public enum BeamRegistrationRequestBuilder {
    public static func request(
        baseURL: URL,
        serviceID: String,
        bearerToken: String,
        registration: BeamDeviceRegistration
    ) throws -> URLRequest {
        let endpoint = baseURL
            .appending(path: "api")
            .appending(path: "services")
            .appending(path: serviceID)
            .appending(path: "devices")
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.httpBody = try JSONEncoder().encode(registration)
        return request
    }
}
