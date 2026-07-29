import SwiftUI

public struct BeamAppHomeView: View {
    public let state: BeamAppHomeViewState

    public init(state: BeamAppHomeViewState) {
        self.state = state
    }

    public var body: some View {
        NavigationStack {
            List {
                Section {
                    VStack(alignment: .leading, spacing: 8) {
                        Text(state.statusTitle)
                            .font(.headline)
                        Text(state.statusDetail)
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                    .padding(.vertical, 4)
                }

                Section("Device") {
                    Label(state.deviceName, systemImage: "iphone")
                    readinessRow(state.notificationLine, ready: state.canReceiveNotifications)
                    readinessRow(state.liveActivityLine, ready: state.canStartLiveActivities)
                    Label(state.permissionLine, systemImage: "bell")
                }
            }
            .navigationTitle(state.title)
        }
    }

    private func readinessRow(_ title: String, ready: Bool) -> some View {
        Label(title, systemImage: ready ? "checkmark.circle" : "exclamationmark.triangle")
            .foregroundStyle(ready ? .primary : .secondary)
    }
}
