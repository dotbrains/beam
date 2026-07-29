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
