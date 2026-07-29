import Foundation
import Testing

@testable import BeamAppCore

@Test func buildsDeviceRegistrationRequest() throws {
    let request = try BeamRegistrationRequestBuilder.request(
        baseURL: try #require(URL(string: "https://beam.example.com")),
        serviceID: "svc_abc",
        bearerToken: "beam_agent_secret",
        registration: BeamDeviceRegistration(
            name: "Nick's iPhone",
            pushToken: "notification_secret_123",
            pushToStartToken: "activity_secret_123"
        )
    )

    #expect(request.url?.absoluteString == "https://beam.example.com/api/services/svc_abc/devices")
    #expect(request.httpMethod == "POST")
    #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer beam_agent_secret")
    #expect(request.value(forHTTPHeaderField: "Content-Type") == "application/json")

    let body = try #require(request.httpBody)
    let fields = try JSONDecoder().decode([String: String].self, from: body)
    #expect(fields["name"] == "Nick's iPhone")
    #expect(fields["platform"] == "ios")
    #expect(fields["pushToken"] == "notification_secret_123")
    #expect(fields["pushToStartToken"] == "activity_secret_123")
}

@Test func decodesTokenSafeDeviceRegistrationResponse() throws {
    let data = Data("""
    {
      "ok": true,
      "device": {
        "id": "dev_ios",
        "name": "Nick's iPhone",
        "platform": "ios",
        "active": true,
        "pushTokenRegistered": true,
        "pushToStartTokenRegistered": true
      }
    }
    """.utf8)

    let response = try JSONDecoder().decode(BeamDeviceRegistrationResponse.self, from: data)

    #expect(response.ok)
    #expect(response.device.id == "dev_ios")
    #expect(response.device.pushTokenRegistered)
    #expect(response.device.pushToStartTokenRegistered)
}

@Test func deviceResponseModelDoesNotExposePushTokens() throws {
    let response = BeamDevice(
        id: "dev_ios",
        name: "Nick's iPhone",
        platform: "ios",
        active: true,
        pushTokenRegistered: true,
        pushToStartTokenRegistered: true
    )
    let mirror = String(describing: response)

    #expect(!mirror.contains("notification_secret_123"))
    #expect(!mirror.contains("activity_secret_123"))
}

@Test func registrarSubmitsRegistrationAndReturnsDevice() async throws {
    let responseBody = Data("""
    {
      "ok": true,
      "device": {
        "id": "dev_ios",
        "name": "Nick's iPhone",
        "platform": "ios",
        "active": true,
        "pushTokenRegistered": true,
        "pushToStartTokenRegistered": false
      }
    }
    """.utf8)
    let transport = MockRegistrationTransport(statusCode: 201, body: responseBody)
    let registrar = BeamDeviceRegistrar(
        baseURL: try #require(URL(string: "https://beam.example.com")),
        serviceID: "svc_abc",
        bearerToken: "beam_agent_secret",
        transport: transport
    )

    let device = try await registrar.register(
        BeamDeviceRegistration(name: "Nick's iPhone", pushToken: "notification_secret_123")
    )
    let request = await transport.lastRequest()

    #expect(device.id == "dev_ios")
    #expect(device.pushTokenRegistered)
    #expect(!device.pushToStartTokenRegistered)
    #expect(request?.url?.absoluteString == "https://beam.example.com/api/services/svc_abc/devices")
    #expect(request?.value(forHTTPHeaderField: "Authorization") == "Bearer beam_agent_secret")
}

@Test func registrarRejectsNonSuccessStatus() async throws {
    let transport = MockRegistrationTransport(statusCode: 402, body: Data("""
    {"ok":false,"error":"payment required","code":"payment_required"}
    """.utf8))
    let registrar = BeamDeviceRegistrar(
        baseURL: try #require(URL(string: "https://beam.example.com")),
        serviceID: "svc_abc",
        bearerToken: "beam_agent_secret",
        transport: transport
    )

    do {
        _ = try await registrar.register(BeamDeviceRegistration(name: "Nick's iPhone"))
        Issue.record("expected registration failure")
    } catch let error as BeamRegistrationError {
        #expect(error == .serverRejected(statusCode: 402))
    }
}

private actor MockRegistrationTransport: BeamHTTPTransport {
    private let statusCode: Int
    private let body: Data
    private var capturedRequest: URLRequest?

    init(statusCode: Int, body: Data) {
        self.statusCode = statusCode
        self.body = body
    }

    func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        capturedRequest = request
        guard let url = request.url,
              let response = HTTPURLResponse(
            url: url,
            statusCode: statusCode,
            httpVersion: nil,
            headerFields: nil
        ) else {
            throw BeamRegistrationError.invalidHTTPResponse
        }
        return (body, response)
    }

    func lastRequest() -> URLRequest? {
        capturedRequest
    }
}
