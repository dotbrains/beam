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

    public init(state: BeamLiveActivityState, sequence: Int? = nil) {
        self.title = state.title
        self.status = state.status
        self.detail = state.detail
        self.progress = state.progress.map { min(max($0, 0), 1) }
        self.symbol = BeamLiveActivitySymbol(rawValue: state.symbol) ?? .terminal
        self.accentColor = state.accentColor
        self.layout = BeamLiveActivityLayout(rawValue: state.style) ?? .standard
        self.privacy = BeamLiveActivityPrivacy(rawValue: state.privacyMode) ?? .standard
        self.sequence = sequence
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
