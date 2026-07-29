import Foundation

public struct BeamActivityContentState: Codable, Hashable, Sendable {
    public let title: String
    public let status: String
    public let detail: String?
    public let progress: Double?
    public let symbol: String
    public let accentColor: String
    public let layout: String
    public let privacyMode: String
    public let sequence: Int?

    public init(
        title: String,
        status: String,
        detail: String? = nil,
        progress: Double? = nil,
        symbol: String,
        accentColor: String,
        layout: String,
        privacyMode: String,
        sequence: Int? = nil
    ) {
        self.title = title
        self.status = status
        self.detail = detail
        self.progress = progress
        self.symbol = symbol
        self.accentColor = accentColor
        self.layout = layout
        self.privacyMode = privacyMode
        self.sequence = sequence
    }

    public init(presentation: BeamLiveActivityPresentation) {
        self.init(
            title: presentation.title,
            status: presentation.status,
            detail: presentation.detail,
            progress: presentation.progress,
            symbol: presentation.symbol.rawValue,
            accentColor: presentation.accentColor,
            layout: presentation.layout.rawValue,
            privacyMode: presentation.privacy.rawValue,
            sequence: presentation.sequence
        )
    }

    public var presentation: BeamLiveActivityPresentation {
        BeamLiveActivityPresentation(
            title: title,
            status: status,
            detail: detail,
            progress: progress,
            symbol: BeamLiveActivitySymbol(rawValue: symbol) ?? .terminal,
            accentColor: accentColor,
            layout: BeamLiveActivityLayout(rawValue: layout) ?? .standard,
            privacy: BeamLiveActivityPrivacy(rawValue: privacyMode) ?? .standard,
            sequence: sequence
        )
    }
}

#if os(iOS)
import ActivityKit

@available(iOS 16.1, *)
public struct BeamActivityAttributes: ActivityAttributes, Equatable, Sendable {
    public typealias ContentState = BeamActivityContentState

    public let activityID: String
    public let key: String?

    public init(activityID: String, key: String? = nil) {
        self.activityID = activityID
        self.key = key
    }
}
#endif

public extension BeamLiveActivityPresentation {
    var activityContentState: BeamActivityContentState {
        BeamActivityContentState(presentation: self)
    }
}
