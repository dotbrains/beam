import Foundation
import Testing

@testable import BeamAppCore

@Test func decodesNotificationPayload() throws {
    let data = Data("""
    {
      "aps": {
        "alert": {
          "title": "Deploys",
          "body": "shipped"
        }
      }
    }
    """.utf8)

    let payload = try BeamPushPayloadDecoder.decode(data)

    #expect(payload.notificationTitle == "Deploys")
    #expect(payload.notificationBody == "shipped")
    #expect(payload.isLiveActivityPush == false)
}

@Test func decodesLiveActivityPayload() throws {
    let data = Data("""
    {
      "aps": {
        "event": "update"
      },
      "content-state": {
        "title": "Deploy",
        "status": "Running",
        "detail": "Compiling",
        "progress": 0.5,
        "symbol": "build",
        "accentColor": "#35C759",
        "style": "ring",
        "privacyMode": "private"
      },
      "beam-sequence": 3
    }
    """.utf8)

    let payload = try BeamPushPayloadDecoder.decode(data)

    #expect(payload.liveActivityEvent == .update)
    #expect(payload.contentState?.title == "Deploy")
    #expect(payload.contentState?.status == "Running")
    #expect(payload.contentState?.progress == 0.5)
    #expect(payload.contentState?.privacyMode == "private")
    #expect(payload.sequence == 3)
}

@Test func payloadModelDoesNotExposeProviderSecrets() throws {
    let data = Data("""
    {
      "aps": {
        "alert": {
          "title": "Deploys",
          "body": "shipped"
        }
      },
      "authorization": "bearer provider_secret",
      "pushToken": "device_secret"
    }
    """.utf8)

    let payload = try BeamPushPayloadDecoder.decode(data)
    let mirror = String(describing: payload)

    #expect(!mirror.contains("provider_secret"))
    #expect(!mirror.contains("device_secret"))
}
