import Testing

@testable import BeamAppCore

#if os(iOS)
@available(iOS 16.1, *)
@Test func activityAttributesCarryStableActivityIdentity() {
    let attributes = BeamActivityAttributes(activityID: "act_deploy", key: "deploy")

    #expect(attributes.activityID == "act_deploy")
    #expect(attributes.key == "deploy")
}
#endif

@Test func activityContentStateMapsFromPresentation() {
    let presentation = BeamLiveActivityPresentation(
        title: "Deploy",
        status: "Building",
        detail: "Step 2",
        progress: 0.42,
        symbol: .build,
        accentColor: "#00ffaa",
        layout: .ring,
        privacy: .private,
        sequence: 7
    )

    let state = presentation.activityContentState

    #expect(state.title == "Deploy")
    #expect(state.status == "Building")
    #expect(state.detail == "Step 2")
    #expect(state.progress == 0.42)
    #expect(state.symbol == "build")
    #expect(state.accentColor == "#00ffaa")
    #expect(state.layout == "ring")
    #expect(state.privacyMode == "private")
    #expect(state.sequence == 7)
}

@Test func activityContentStateRoundTripsToPresentation() {
    let state = BeamActivityContentState(
        title: "Deploy",
        status: "Testing",
        detail: nil,
        progress: 1.4,
        symbol: "success",
        accentColor: "#66dd99",
        layout: "hero",
        privacyMode: "standard",
        sequence: 8
    )

    let presentation = state.presentation

    #expect(presentation.title == "Deploy")
    #expect(presentation.status == "Testing")
    #expect(presentation.progress == 1)
    #expect(presentation.symbol == .success)
    #expect(presentation.layout == .hero)
    #expect(presentation.privacy == .standard)
    #expect(presentation.sequence == 8)
}
