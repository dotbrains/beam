# iOS App Core

Beam's iOS surface starts as a Swift package under `ios/` so push payload
handling can be compiled and tested without requiring a signing identity. The
package models the APNs payloads emitted by `beam provider-worker --mode apns`
and keeps provider credentials outside app-visible data structures.

```mermaid
flowchart LR
  app[iOS app] -->|register tokens| api[Device registration API]
  api --> service[Service device list]
  worker[APNs worker] -->|alert payload| notification[Notification decoder]
  worker -->|Live Activity payload| activity[Live Activity decoder]
  notification --> ui[Notification UI]
  activity --> live[Activity state]
  secrets[Provider and device tokens] -. excluded .-> ui
  secrets -. excluded .-> live
```

## Package Layout

```text
ios/
  Package.swift
  Sources/BeamAppCore/
  Tests/BeamAppCoreTests/
```

`BeamAppCore` currently decodes:

- one-shot notification payloads with `aps.alert.title` and `aps.alert.body`
- Live Activity payloads with `aps.event`, `content-state`, and
  `beam-sequence`
- the Beam Live Activity fields used by the server contract: title, status,
  detail, progress, symbol, accent color, layout style, and privacy mode
- device registration requests for iOS notification tokens and Live Activity
  push-to-start tokens
- token-safe device registration responses with `pushTokenRegistered` and
  `pushToStartTokenRegistered`

The package intentionally ignores APNs provider headers, bearer tokens, and
device push tokens when decoding app-visible responses. Provider credentials
belong in the provider worker and must not appear in iOS app state, logs, or UI.
Device push tokens are sent only in registration requests and are represented
as boolean registration flags after the server accepts them.

## Device Registration

The app-core registration builder creates:

```text
POST /api/services/:serviceId/devices
Authorization: Bearer <agent token>
Content-Type: application/json
```

with the server contract:

```json
{
  "name": "Nick's iPhone",
  "platform": "ios",
  "pushToken": "...",
  "pushToStartToken": "..."
}
```

Registration responses are decoded into token-safe device records. They expose
stable IDs, active state, and registration booleans rather than echoing APNs
token material.

## Verification

Run the app-core tests locally:

```sh
cd ios
swift test
```

CI runs the same command on macOS. A future app slice should add the SwiftUI
target, APNs permission/token collection, and ActivityKit rendering on top of
this tested payload and registration boundary.
