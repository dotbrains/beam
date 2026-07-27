# Architecture

Beam has three surfaces: webhook callers, authenticated CLI clients, and device
callbacks. The current implementation keeps state in memory; the target
architecture swaps that package behind durable repositories and provider
adapters.

## Context

```mermaid
C4Context
  title Beam context
  Person(user, "User", "Receives notifications and answers prompts")
  System_Ext(ci, "Automation", "CI, cron, monitors, coding agents")
  System(beam, "Beam", "Webhook notification and interaction service")
  System_Ext(push, "Push provider", "APNs, Expo, or another delivery adapter")
  Rel(ci, beam, "POST webhook JSON")
  Rel(beam, push, "Send notification/activity pushes")
  Rel(push, user, "Deliver to phone")
  Rel(user, beam, "Answer prompt")
```

## Components

```mermaid
flowchart TB
  subgraph API
    webhook[Webhook routes]
    agent[Agent routes]
    health[Health routes]
  end
  subgraph Domain
    auth[Token and scope checks]
    limits[Rate and allowance gates]
    idem[Idempotency]
    events[Event service]
    activities[Activity service]
    callbacks[Callback scheduler]
  end
  subgraph Storage
    db[(Database)]
    queue[(Queue)]
  end
  subgraph Delivery
    router[Device router]
    provider[Push provider adapter]
  end
  webhook --> auth --> limits
  agent --> auth
  limits --> idem
  idem --> events
  idem --> activities
  events --> router --> provider
  activities --> router
  events --> callbacks --> queue
  events --> db
  activities --> db
```

## Request lifecycle

```mermaid
sequenceDiagram
  participant Caller
  participant API
  participant Store
  participant Router
  participant Provider
  Caller->>API: POST /hooks/:token
  API->>Store: validate token and idempotency
  Store-->>API: reserved event
  API->>Router: choose target devices
  Router->>Provider: send push requests
  Provider-->>Router: accepted/rejected
  Router-->>Store: delivery result
  API-->>Caller: ok, eventId, delivered
```

## State boundaries

```mermaid
erDiagram
  ACCOUNT ||--o{ SERVICE : owns
  ACCOUNT ||--o{ DEVICE : registers
  SERVICE ||--o{ WEBHOOK_TOKEN : rotates
  SERVICE ||--o{ EVENT : creates
  EVENT ||--o| INTERACTION : may_have
  EVENT ||--o{ CALLBACK_ATTEMPT : schedules
  SERVICE ||--o{ LIVE_ACTIVITY : starts
  LIVE_ACTIVITY ||--o{ ACTIVITY_UPDATE : records
  SERVICE ||--o{ IDEMPOTENCY_RECORD : scopes
```

Service management is intentionally token-safe: public service views include
metadata and device counts, while plaintext webhook tokens are emitted only at
creation or rotation time. Rotating a token removes the old token from the
webhook lookup map, so old webhook URLs immediately return `404`.

Devices are service-scoped records with stable IDs, platform, active state, and
timestamps. Notification routing validates requested IDs against active devices;
deactivated devices remain visible for history but no longer accept routed
notifications.

## Current durable storage

Beam now has a SQLite-backed backend for the development server. The current
implementation persists a JSON domain snapshot after successful mutating
operations and loads that snapshot on startup. This is intentionally smaller
than the target normalized schema, but it satisfies the first durable-storage
step: services, events, activities, and idempotency records survive a restart
and migrations are checked into the repo.

```mermaid
flowchart LR
  api[HTTP API] --> backend[Backend interface]
  backend --> mem[Domain Store]
  backend --> sqlite[(SQLite snapshots)]
  migrations[Checked-in migrations] --> sqlite
```

## LOC budget

Beam should avoid large multi-responsibility files. The current budget is 500
lines by default with explicit documentation exceptions in
`scripts/file-size-budgets.json`. Flat directories default to 12 direct files,
with narrow exceptions for the root and `cmd`.
