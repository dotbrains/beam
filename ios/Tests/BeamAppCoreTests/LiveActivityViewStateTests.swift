import Testing

@testable import BeamAppCore

@Test func liveActivityViewStateMapsPresentationForRendering() {
    let presentation = BeamLiveActivityPresentation(
        title: "Deploy",
        status: "Building",
        detail: "Step 2",
        progress: 0.456,
        symbol: .build,
        accentColor: "#00ffaa",
        layout: .ring,
        privacy: .standard,
        sequence: 12
    )

    let state = BeamLiveActivityViewState(presentation: presentation)

    #expect(state.title == "Deploy")
    #expect(state.status == "Building")
    #expect(state.detail == "Step 2")
    #expect(state.progress == 0.456)
    #expect(state.progressLabel == "46%")
    #expect(state.primaryLine == "Building 46%")
    #expect(state.symbolName == "hammer")
    #expect(state.accentColor == "#00ffaa")
    #expect(state.layout == .ring)
    #expect(!state.isPrivate)
    #expect(state.sequenceLabel == "#12")
}

@Test func liveActivityViewStateRedactsPrivateDetail() {
    let presentation = BeamLiveActivityPresentation(
        title: "Deploy",
        status: "Running",
        detail: "Secret environment",
        progress: nil,
        symbol: .terminal,
        accentColor: "#ffffff",
        layout: .terminal,
        privacy: .private
    )

    let state = BeamLiveActivityViewState(presentation: presentation)

    #expect(state.detail == nil)
    #expect(state.primaryLine == "Running")
    #expect(state.symbolName == "terminal")
    #expect(state.isPrivate)
}

@Test func liveActivityViewStateMapsEveryBeamSymbolToSystemSymbol() {
    let expected: [(BeamLiveActivitySymbol, String)] = [
        (.terminal, "terminal"),
        (.code, "chevron.left.forwardslash.chevron.right"),
        (.build, "hammer"),
        (.success, "checkmark.circle"),
        (.warning, "exclamationmark.triangle")
    ]

    for (symbol, systemName) in expected {
        let state = BeamLiveActivityViewState(presentation: BeamLiveActivityPresentation(
            title: "Deploy",
            status: "Running",
            symbol: symbol,
            accentColor: "#ffffff",
            layout: .standard,
            privacy: .standard
        ))

        #expect(state.symbolName == systemName)
    }
}

@Test func liveActivityViewStateMapsFromActivityContentState() {
    let contentState = BeamActivityContentState(
        title: "Deploy",
        status: "Succeeded",
        detail: "Complete",
        progress: 1,
        symbol: "success",
        accentColor: "#44cc88",
        layout: "hero",
        privacyMode: "standard",
        sequence: 4
    )

    let state = BeamLiveActivityViewState(contentState: contentState)

    #expect(state.title == "Deploy")
    #expect(state.progressLabel == "100%")
    #expect(state.symbolName == "checkmark.circle")
    #expect(state.layout == .hero)
    #expect(state.sequenceLabel == "#4")
}
