import BeamAppCore
import SwiftUI
import WidgetKit

#if canImport(ActivityKit)
import ActivityKit

@available(iOS 16.1, *)
struct BeamLiveActivityWidget: Widget {
    private let contentStateType = BeamActivityContentState.self

    var body: some WidgetConfiguration {
        ActivityConfiguration(for: BeamActivityAttributes.self) { context in
            let _ = contentStateType
            BeamLiveActivityView(
                state: BeamLiveActivityViewState(
                    presentation: context.state.presentation
                )
            )
        } dynamicIsland: { context in
            let state = BeamLiveActivityViewState(
                presentation: context.state.presentation
            )

            return DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    BeamLiveActivityCompactView(state: state)
                }
                DynamicIslandExpandedRegion(.trailing) {
                    BeamLiveActivityMinimalView(state: state)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    BeamLiveActivityView(state: state)
                }
            } compactLeading: {
                BeamLiveActivityMinimalView(state: state)
            } compactTrailing: {
                BeamLiveActivityCompactView(state: state)
            } minimal: {
                BeamLiveActivityMinimalView(state: state)
            }
        }
    }
}
#endif
