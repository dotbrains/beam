import Testing

@testable import BeamAppCore

#if canImport(UserNotifications)
import UserNotifications

@Test func mapsUserNotificationAuthorizationStatuses() {
    #expect(BeamUserNotificationAuthorizer.authorization(from: .notDetermined) == .notDetermined)
    #expect(BeamUserNotificationAuthorizer.authorization(from: .denied) == .denied)
    #expect(BeamUserNotificationAuthorizer.authorization(from: .authorized) == .authorized)
    #expect(BeamUserNotificationAuthorizer.authorization(from: .provisional) == .provisional)
    #if os(iOS)
    #expect(BeamUserNotificationAuthorizer.authorization(from: .ephemeral) == .ephemeral)
    #endif
}

@Test func mapsBeamNotificationOptionsToUserNotificationOptions() {
    let options: BeamNotificationAuthorizationOptions = [.alert, .badge]
    let userOptions = options.userNotificationOptions

    #expect(userOptions.contains(.alert))
    #expect(userOptions.contains(.badge))
    #expect(!userOptions.contains(.sound))
}
#endif
