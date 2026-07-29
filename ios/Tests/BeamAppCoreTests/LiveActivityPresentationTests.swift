import Foundation
import Testing

@testable import BeamAppCore

@Test func mapsLiveActivityStateToPresentation() {
    let presentation = BeamLiveActivityPresentation(
        state: BeamLiveActivityState(
            title: "Deploy",
            status: "Running",
            detail: "Compiling",
            progress: 0.426,
            symbol: "build",
            accentColor: "#35C759",
            style: "ring",
            privacyMode: "private"
        ),
        sequence: 7
    )

    #expect(presentation.title == "Deploy")
    #expect(presentation.status == "Running")
    #expect(presentation.detail == "Compiling")
    #expect(presentation.symbol == .build)
    #expect(presentation.layout == .ring)
    #expect(presentation.privacy == .private)
    #expect(presentation.isPrivate)
    #expect(presentation.sequence == 7)
    #expect(presentation.percentComplete == 43)
}

@Test func clampsProgressForPresentation() {
    let high = BeamLiveActivityPresentation(state: BeamLiveActivityState(
        title: "Deploy",
        status: "Running",
        progress: 1.4,
        symbol: "success",
        accentColor: "#35C759",
        style: "hero",
        privacyMode: "standard"
    ))
    let low = BeamLiveActivityPresentation(state: BeamLiveActivityState(
        title: "Deploy",
        status: "Running",
        progress: -0.2,
        symbol: "warning",
        accentColor: "#FFCC00",
        style: "standard",
        privacyMode: "standard"
    ))

    #expect(high.progress == 1)
    #expect(high.percentComplete == 100)
    #expect(low.progress == 0)
    #expect(low.percentComplete == 0)
}

@Test func fallsBackForUnknownPresentationValues() {
    let presentation = BeamLiveActivityPresentation(state: BeamLiveActivityState(
        title: "Deploy",
        status: "Running",
        symbol: "unknown",
        accentColor: "#888888",
        style: "unknown",
        privacyMode: "unknown"
    ))

    #expect(presentation.symbol == .terminal)
    #expect(presentation.layout == .standard)
    #expect(presentation.privacy == .standard)
}

@Test func derivesPresentationFromPushPayload() {
    let payload = BeamPushPayload(
        aps: APS(event: "update"),
        contentState: BeamLiveActivityState(
            title: "Deploy",
            status: "Running",
            progress: 0.5,
            symbol: "terminal",
            accentColor: "#FFFFFF",
            style: "terminal",
            privacyMode: "standard"
        ),
        sequence: 3
    )

    let presentation = payload.liveActivityPresentation

    #expect(presentation?.layout == .terminal)
    #expect(presentation?.percentComplete == 50)
    #expect(presentation?.sequence == 3)
}
