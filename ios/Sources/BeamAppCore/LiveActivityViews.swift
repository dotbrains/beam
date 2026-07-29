import SwiftUI

public struct BeamLiveActivityView: View {
    public let state: BeamLiveActivityViewState

    public init(state: BeamLiveActivityViewState) {
        self.state = state
    }

    public var body: some View {
        switch state.layout {
        case .ring:
            ringLayout
        case .hero:
            heroLayout
        case .terminal:
            terminalLayout
        case .steps:
            stepsLayout
        case .standard:
            standardLayout
        }
    }

    private var standardLayout: some View {
        VStack(alignment: .leading, spacing: 8) {
            header
            progress
            detail
        }
        .padding(14)
    }

    private var ringLayout: some View {
        HStack(spacing: 12) {
            ProgressView(value: state.progress ?? 0, total: 1)
                .progressViewStyle(.circular)
                .tint(accent)
            VStack(alignment: .leading, spacing: 4) {
                header
                detail
            }
        }
        .padding(14)
    }

    private var heroLayout: some View {
        VStack(alignment: .leading, spacing: 10) {
            Image(systemName: state.symbolName)
                .font(.title2)
                .foregroundStyle(accent)
            header
            progress
            detail
        }
        .padding(16)
    }

    private var terminalLayout: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text("$ \(state.primaryLine)")
                .font(.system(.callout, design: .monospaced))
                .fontWeight(.semibold)
            detail
        }
        .padding(14)
    }

    private var stepsLayout: some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: state.symbolName)
                .foregroundStyle(accent)
            VStack(alignment: .leading, spacing: 5) {
                header
                progress
                detail
            }
        }
        .padding(14)
    }

    private var header: some View {
        HStack(spacing: 8) {
            Image(systemName: state.symbolName)
                .foregroundStyle(accent)
            VStack(alignment: .leading, spacing: 2) {
                Text(state.title)
                    .font(.headline)
                    .lineLimit(1)
                Text(state.primaryLine)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer(minLength: 8)
            if let sequenceLabel = state.sequenceLabel {
                Text(sequenceLabel)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private var progress: some View {
        if let progress = state.progress {
            ProgressView(value: progress, total: 1)
                .tint(accent)
        }
    }

    @ViewBuilder
    private var detail: some View {
        if let detail = state.detail {
            Text(detail)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(2)
        }
    }

    private var accent: Color {
        Color(beamHex: state.accentColor)
    }
}

public struct BeamLiveActivityCompactView: View {
    public let state: BeamLiveActivityViewState

    public init(state: BeamLiveActivityViewState) {
        self.state = state
    }

    public var body: some View {
        HStack(spacing: 6) {
            Image(systemName: state.symbolName)
                .foregroundStyle(Color(beamHex: state.accentColor))
            Text(state.progressLabel ?? state.status)
                .font(.caption)
                .lineLimit(1)
        }
    }
}

public struct BeamLiveActivityMinimalView: View {
    public let state: BeamLiveActivityViewState

    public init(state: BeamLiveActivityViewState) {
        self.state = state
    }

    public var body: some View {
        Image(systemName: state.symbolName)
            .foregroundStyle(Color(beamHex: state.accentColor))
    }
}

private extension Color {
    init(beamHex: String) {
        let value = beamHex.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        guard value.count == 6, let number = Int(value, radix: 16) else {
            self = .accentColor
            return
        }
        let red = Double((number >> 16) & 0xff) / 255
        let green = Double((number >> 8) & 0xff) / 255
        let blue = Double(number & 0xff) / 255
        self = Color(red: red, green: green, blue: blue)
    }
}
