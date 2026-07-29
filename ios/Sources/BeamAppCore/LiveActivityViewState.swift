import Foundation

public struct BeamLiveActivityViewState: Equatable, Sendable {
    public let title: String
    public let status: String
    public let detail: String?
    public let progress: Double?
    public let progressLabel: String?
    public let symbolName: String
    public let accentColor: String
    public let layout: BeamLiveActivityLayout
    public let isPrivate: Bool
    public let sequenceLabel: String?

    public init(presentation: BeamLiveActivityPresentation) {
        self.title = presentation.title
        self.status = presentation.status
        self.detail = presentation.isPrivate ? nil : presentation.detail
        self.progress = presentation.progress
        self.progressLabel = presentation.percentComplete.map { "\($0)%" }
        self.symbolName = Self.systemSymbolName(for: presentation.symbol)
        self.accentColor = presentation.accentColor
        self.layout = presentation.layout
        self.isPrivate = presentation.isPrivate
        self.sequenceLabel = presentation.sequence.map { "#\($0)" }
    }

    public init(contentState: BeamActivityContentState) {
        self.init(presentation: contentState.presentation)
    }

    public var primaryLine: String {
        if let progressLabel {
            "\(status) \(progressLabel)"
        } else {
            status
        }
    }

    private static func systemSymbolName(for symbol: BeamLiveActivitySymbol) -> String {
        switch symbol {
        case .terminal:
            "terminal"
        case .code:
            "chevron.left.forwardslash.chevron.right"
        case .build:
            "hammer"
        case .success:
            "checkmark.circle"
        case .warning:
            "exclamationmark.triangle"
        }
    }
}
