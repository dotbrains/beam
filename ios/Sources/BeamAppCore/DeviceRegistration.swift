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

public enum BeamRegistrationError: Error, Equatable, Sendable {
    case invalidHTTPResponse
    case serverRejected(statusCode: Int)
}

public protocol BeamHTTPTransport: Sendable {
    func data(for request: URLRequest) async throws -> (Data, URLResponse)
}

extension URLSession: BeamHTTPTransport {}

public struct BeamDeviceRegistrar: Sendable {
    public let baseURL: URL
    public let serviceID: String
    public let bearerToken: String
    public let transport: any BeamHTTPTransport

    public init(
        baseURL: URL,
        serviceID: String,
        bearerToken: String,
        transport: any BeamHTTPTransport = URLSession.shared
    ) {
        self.baseURL = baseURL
        self.serviceID = serviceID
        self.bearerToken = bearerToken
        self.transport = transport
    }

    public func register(_ registration: BeamDeviceRegistration) async throws -> BeamDevice {
        let request = try BeamRegistrationRequestBuilder.request(
            baseURL: baseURL,
            serviceID: serviceID,
            bearerToken: bearerToken,
            registration: registration
        )
        let (data, response) = try await transport.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw BeamRegistrationError.invalidHTTPResponse
        }
        guard 200..<300 ~= httpResponse.statusCode else {
            throw BeamRegistrationError.serverRejected(statusCode: httpResponse.statusCode)
        }
        return try JSONDecoder().decode(BeamDeviceRegistrationResponse.self, from: data).device
    }
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
