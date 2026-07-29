import SwiftUI
import Testing

@testable import BeamAppCore

@MainActor
@Test func liveActivityViewsCompileForEveryLayout() {
    for layout in BeamLiveActivityLayout.allCasesForTesting {
        let view = BeamLiveActivityView(state: viewState(layout: layout))

        assertViewType(view)
    }
}

@MainActor
@Test func compactAndMinimalLiveActivityViewsCompile() {
    let state = viewState(layout: .standard)

    assertViewType(BeamLiveActivityCompactView(state: state))
    assertViewType(BeamLiveActivityMinimalView(state: state))
}

@MainActor
@Test func liveActivityViewsAcceptInvalidAccentColorFallback() {
    let state = BeamLiveActivityViewState(presentation: BeamLiveActivityPresentation(
        title: "Deploy",
        status: "Running",
        progress: 0.2,
        symbol: .warning,
        accentColor: "not-a-color",
        layout: .hero,
        privacy: .standard
    ))

    assertViewType(BeamLiveActivityView(state: state))
    assertViewType(BeamLiveActivityCompactView(state: state))
    assertViewType(BeamLiveActivityMinimalView(state: state))
}

private func assertViewType<V: View>(_ view: V) {
    _ = V.self
}

private func viewState(layout: BeamLiveActivityLayout) -> BeamLiveActivityViewState {
    BeamLiveActivityViewState(presentation: BeamLiveActivityPresentation(
        title: "Deploy",
        status: "Building",
        detail: "Step 2",
        progress: 0.45,
        symbol: .build,
        accentColor: "#00ffaa",
        layout: layout,
        privacy: .standard,
        sequence: 3
    ))
}

private extension BeamLiveActivityLayout {
    static let allCasesForTesting: [BeamLiveActivityLayout] = [
        .standard,
        .ring,
        .hero,
        .terminal,
        .steps
    ]
}
