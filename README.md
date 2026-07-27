# beam

![beam](./assets/og-image.svg)

[![CI](https://github.com/dotbrains/beam/actions/workflows/ci.yml/badge.svg)](https://github.com/dotbrains/beam/actions/workflows/ci.yml)
[![Release](https://github.com/dotbrains/beam/actions/workflows/release.yml/badge.svg)](https://github.com/dotbrains/beam/actions/workflows/release.yml)
[![License: PolyForm Shield 1.0.0](https://img.shields.io/badge/License-PolyForm%20Shield%201.0.0-blue.svg)](https://polyformproject.org/licenses/shield/1.0.0/)

![Go](https://img.shields.io/badge/-Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Cobra](https://img.shields.io/badge/-Cobra-00ADD8?style=flat-square&logo=go&logoColor=white)

![macOS](https://img.shields.io/badge/-macOS-000000?style=flat-square&logo=apple&logoColor=white)
![Linux](https://img.shields.io/badge/-Linux-FCC624?style=flat-square&logo=linux&logoColor=black)

Beam is a clean-room webhook notification service and CLI. It accepts secret webhook URLs, records notification events, supports idempotent retries, models interactive approval prompts, and exposes Live Activity-style state for scripts and agents.

## Quick Start

```sh
# Install
go install github.com/dotbrains/beam@latest

# Show version
beam --version

# Initialize CLI config
beam config init

# Run the local API
beam serve --addr 127.0.0.1:8080

# Send a notification through the default dev token
beam notify "Production deployed" --title CI --url https://ci.example.com/builds/48
```

## How It Works

1. Start `beam serve`; it registers a local development service at `dev_token`.
2. POST JSON to `/hooks/:token` or use `beam notify`.
3. Read event state from `/hooks/:token/events/:eventId`.
4. Start, update, read, and end Live Activity state under `/hooks/:token/live-activities`.

The current implementation is intentionally self-contained and in-memory. It is suitable for local integrations, tests, and as the foundation for a hosted backend with persistent services, devices, APNs/Expo delivery, auth, and billing gates.

## Installation

### Via `go install`

```sh
go install github.com/dotbrains/beam@latest
```

### Via Homebrew

```sh
brew tap dotbrains/tap
brew install --cask beam
```

### Via GitHub Release

```sh
gh release download --repo dotbrains/beam --pattern 'beam_darwin_arm64.tar.gz' --dir /tmp
tar -xzf /tmp/beam_darwin_arm64.tar.gz -C /usr/local/bin

```

### From source

```sh
git clone https://github.com/dotbrains/beam.git
cd beam
make install
```

## Configuration

```sh
# Create default config
beam config init

# Config lives at ~/.config/beam/config.yaml
```

See [SPEC.md](SPEC.md) for the full config format.

## Commands

| Command | Description |
|---|---|
| `beam serve` | Run the webhook API |
| `beam notify <body>` | Send a one-shot notification |
| `beam ask <prompt>` | Create an interactive prompt |
| `beam activity start` | Start Live Activity state |
| `beam activity update <id-or-key>` | Patch Live Activity state |
| `beam activity end <id-or-key>` | End Live Activity state |
| `beam activity get <id-or-key>` | Read Live Activity state |
| `beam config init` | Create default config file |

## API Examples

```sh
curl -X POST http://127.0.0.1:8080/hooks/dev_token \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: deploy-184' \
  -d '{"title":"CI","body":"Production deployed successfully."}'
```

```sh
curl -X POST http://127.0.0.1:8080/hooks/dev_token/live-activities \
  -H 'Content-Type: application/json' \
  -d '{"key":"deploy","title":"Deploy #184","status":"Building","progress":0}'
```

## Dependencies

- **[Go](https://go.dev/)** >= 1.24

## License

This project is licensed under the [PolyForm Shield License 1.0.0](https://polyformproject.org/licenses/shield/1.0.0/) — see [LICENSE](LICENSE) for details.
