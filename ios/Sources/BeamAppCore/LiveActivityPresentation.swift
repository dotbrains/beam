import Foundation

public struct BeamLiveActivityPresentation: Equatable, Sendable {
    public let title: String
    public let status: String
    public let detail: String?
    public let progress: Double?
    public let symbol: BeamLiveActivitySymbol
    public let accentColor: String
    public let layout: BeamLiveActivityLayout
    public let privacy: BeamLiveActivityPrivacy
    public let sequence: Int?

    public init(
        title: String,
        status: String,
        detail: String? = nil,
        progress: Double? = nil,
        symbol: BeamLiveActivitySymbol,
        accentColor: String,
        layout: BeamLiveActivityLayout,
        privacy: BeamLiveActivityPrivacy,
        sequence: Int? = nil
    ) {
        self.title = title
        self.status = status
        self.detail = detail
        self.progress = progress.map { min(max($0, 0), 1) }
        self.symbol = symbol
        self.accentColor = accentColor
        self.layout = layout
        self.privacy = privacy
        self.sequence = sequence
    }

    public init(state: BeamLiveActivityState, sequence: Int? = nil) {
        self.init(
            title: state.title,
            status: state.status,
            detail: state.detail,
            progress: state.progress,
            symbol: BeamLiveActivitySymbol(rawValue: state.symbol) ?? .terminal,
            accentColor: state.accentColor,
            layout: BeamLiveActivityLayout(rawValue: state.style) ?? .standard,
            privacy: BeamLiveActivityPrivacy(rawValue: state.privacyMode) ?? .standard,
            sequence: sequence
        )
    }

    public var percentComplete: Int? {
        progress.map { Int(($0 * 100).rounded()) }
    }

    public var isPrivate: Bool {
        privacy == .private
    }
}

public enum BeamLiveActivitySymbol: String, Equatable, Sendable {
    case terminal
    case code
    case build
    case success
    case warning
}

public enum BeamLiveActivityLayout: String, Equatable, Sendable {
    case standard
    case ring
    case hero
    case terminal
    case steps
}

public enum BeamLiveActivityPrivacy: String, Equatable, Sendable {
    case standard
    case `private`
}

public extension BeamPushPayload {
    var liveActivityPresentation: BeamLiveActivityPresentation? {
        contentState.map { BeamLiveActivityPresentation(state: $0, sequence: sequence) }
    }
}
