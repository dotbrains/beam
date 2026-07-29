import Foundation

public struct BeamPushPayload: Decodable, Equatable, Sendable {
    public let aps: APS
    public let contentState: BeamLiveActivityState?
    public let sequence: Int?

    public init(aps: APS, contentState: BeamLiveActivityState? = nil, sequence: Int? = nil) {
        self.aps = aps
        self.contentState = contentState
        self.sequence = sequence
    }

    enum CodingKeys: String, CodingKey {
        case aps
        case contentState = "content-state"
        case sequence = "beam-sequence"
    }

    public var notificationTitle: String? {
        aps.alert?.title
    }

    public var notificationBody: String? {
        aps.alert?.body
    }

    public var liveActivityEvent: BeamLiveActivityEvent? {
        aps.event.flatMap(BeamLiveActivityEvent.init(rawValue:))
    }

    public var isLiveActivityPush: Bool {
        liveActivityEvent != nil
    }
}

public struct APS: Decodable, Equatable, Sendable {
    public let alert: APSAlert?
    public let event: String?

    public init(alert: APSAlert? = nil, event: String? = nil) {
        self.alert = alert
        self.event = event
    }
}

public struct APSAlert: Decodable, Equatable, Sendable {
    public let title: String?
    public let body: String?

    public init(title: String? = nil, body: String? = nil) {
        self.title = title
        self.body = body
    }
}

public enum BeamLiveActivityEvent: String, Decodable, Equatable, Sendable {
    case start
    case update
    case end
}

public struct BeamLiveActivityState: Decodable, Equatable, Sendable {
    public let title: String
    public let status: String
    public let detail: String?
    public let progress: Double?
    public let symbol: String
    public let accentColor: String
    public let style: String
    public let privacyMode: String

    public init(
        title: String,
        status: String,
        detail: String? = nil,
        progress: Double? = nil,
        symbol: String,
        accentColor: String,
        style: String,
        privacyMode: String
    ) {
        self.title = title
        self.status = status
        self.detail = detail
        self.progress = progress
        self.symbol = symbol
        self.accentColor = accentColor
        self.style = style
        self.privacyMode = privacyMode
    }
}

public enum BeamPushPayloadDecoder {
    public static func decode(_ data: Data) throws -> BeamPushPayload {
        try JSONDecoder().decode(BeamPushPayload.self, from: data)
    }
}
