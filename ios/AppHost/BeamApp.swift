import BeamAppCore
import SwiftUI

@main
struct BeamApp: App {
    @State private var deviceState = BeamAppDeviceState()

    private let notificationAuthorizer = BeamUserNotificationAuthorizer()
    private let remoteNotificationRegistrar = BeamUIApplicationRemoteNotificationRegistrar()
    private typealias TokenCoordinator = BeamAPNSTokenCoordinator<BeamDeviceRegistrar>

    var body: some Scene {
        WindowGroup {
            BeamAppHomeView(state: homeViewState)
                .task {
                    _ = BeamNotificationPermissionCoordinator(
                        authorizer: notificationAuthorizer,
                        remoteRegistrar: remoteNotificationRegistrar
                    )
                }
        }
    }

    private var homeViewState: BeamAppHomeViewState {
        BeamAppHomeViewState(
            title: "Beam",
            serviceName: "Not connected",
            deviceName: "This iPhone",
            deviceState: deviceState
        )
    }
}
