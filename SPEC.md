# beam specification

![beam](./assets/og-image.svg)

[![CI](https://github.com/dotbrains/beam/actions/workflows/ci.yml/badge.svg)](https://github.com/dotbrains/beam/actions/workflows/ci.yml)
[![Release](https://github.com/dotbrains/beam/actions/workflows/release.yml/badge.svg)](https://github.com/dotbrains/beam/actions/workflows/release.yml)
[![License: PolyForm Shield 1.0.0](https://img.shields.io/badge/License-PolyForm%20Shield%201.0.0-blue.svg)](https://polyformproject.org/licenses/shield/1.0.0/)

![Go](https://img.shields.io/badge/-Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Cobra](https://img.shields.io/badge/-Cobra-00ADD8?style=flat-square&logo=go&logoColor=white)

![macOS](https://img.shields.io/badge/-macOS-000000?style=flat-square&logo=apple&logoColor=white)
![Linux](https://img.shields.io/badge/-Linux-FCC624?style=flat-square&logo=linux&logoColor=black)

Beam turns webhook requests into notification events and exposes a CLI for scripted notifications, prompts, and Live Activity-style progress state.

## Problem

Scripts, CI jobs, coding agents, and monitors often need a tiny HTTP surface for human-facing alerts and approvals. Beam provides that surface without requiring every caller to know device details or own retry semantics.

## Configuration

`beam` reads its configuration from `~/.config/beam/config.yaml`. If the file does not exist, built-in defaults are used.

### Config file format

```yaml
api_url: http://127.0.0.1:8080
token: dev_token
```

### `beam config init`

Scaffolds a config file with the built-in defaults:

```
$ beam config init
✓ Wrote default config to ~/.config/beam/config.yaml
Edit the file to customize settings.
```

Refuses to overwrite an existing file unless `--force` is passed.

## Commands

### `beam serve`

Runs an HTTP server. The development server registers `dev_token` with one synthetic device so local sends return `delivered: 1`.

### Notification API

`POST /hooks/:token`

Required JSON field:

| Field | Type | Notes |
|---|---|---|
| `body` | string | Required, trimmed, 1 to 2,000 characters |
| `title` | string | Optional sender title |
| `imageUrl` | string | Optional sender image |
| `url` | string | Optional tap destination |
| `deviceIds` | string[] | Accepted in the schema for future routing |
| `response` | object | Optional interactive response request |

`Idempotency-Key` replays the original event when the payload matches and returns `409` when the same key is reused with a different payload.

`GET /hooks/:token/events/:eventId` reads an event. `POST /hooks/:token/events/:eventId/cancel` cancels a pending interactive response.

### Activity API

`POST /hooks/:token/live-activities` starts an activity. `PATCH /hooks/:token/live-activities/:id` merges partial state. `GET /hooks/:token/live-activities/:id` reads state. `POST /hooks/:token/live-activities/:id/end` marks it ended.

Core fields:

| Field | Type | Notes |
|---|---|---|
| `title` | string | Required on start |
| `status` | string | Required on start |
| `detail` | string | Optional secondary line |
| `progress` | number | Optional 0 to 1 progress |
| `symbol` | string | Defaults to `terminal` |
| `accentColor` | string | Defaults to `#5ED8B7` |
| `style` | string | Defaults to `standard` |
| `key` | string | Stable alias for scripts |
| `ifSequence` | number | Conditional update guard |

## Architecture

```
main.go                           Entry point, version injection via ldflags
cmd/
  root.go                         Cobra root command + subcommand registration
  cmd_test.go                     Command-level tests
internal/
  config/
    config.go                     YAML config: Load, Save, defaults
    config_test.go
  beam/
    server.go                     HTTP routes
    store.go                      In-memory service, event, idempotency, and activity state
    server_test.go
  exec/
    executor.go                   CommandExecutor interface for testability
    executor_test.go
```

### Adding new subcommands

1. Create `cmd/<name>.go` with a `newXxxCmd()` factory function.
2. Register it in `root.go` via `root.AddCommand(newXxxCmd())`.
3. Add tests in `cmd/cmd_test.go`.

### Adding internal packages

1. Create a new directory under `internal/<package>/`.
2. Include `*_test.go` files alongside source files.
3. Use the `exec.CommandExecutor` interface for any shell-out logic.

## Testing

```sh
# Run all tests
make test

# Run with coverage report
make cover

# Lint
make lint

```

Tests use `t.TempDir()` and `t.Setenv()` for isolation. The `exec.MockExecutor` pattern is used to test code that shells out to external commands.

## Release

Releases are triggered by pushing a git tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

This triggers the release workflow which:
1. Runs tests and lint.
2. Builds via GoReleaser for darwin/linux × amd64/arm64.
3. Publishes a GitHub release with binaries.
4. Updates the Homebrew tap at `dotbrains/homebrew-tap`.
