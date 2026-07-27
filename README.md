# beam

[![CI](https://github.com/dotbrains/beam/actions/workflows/ci.yml/badge.svg)](https://github.com/dotbrains/beam/actions/workflows/ci.yml)
[![License: PolyForm Shield 1.0.0](https://img.shields.io/badge/license-PolyForm%20Shield%201.0.0-blue.svg)](LICENSE)
[![Platform: macOS + Linux + Windows](https://img.shields.io/badge/platform-macOS%20%2B%20Linux%20%2B%20Windows-lightgrey.svg)](docs/getting-started.md)
[![Go](https://img.shields.io/badge/go-1.24+-00ADD8.svg)](go.mod)

Beam is a clean-room webhook notification service and CLI for scripted alerts,
interactive approvals, and Live Activity-style progress state.

```console
$ beam serve
beam listening on 127.0.0.1:8080

$ beam notify "Production deployed" --title CI --url https://ci.example.com/builds/48
{"delivered":1,"eventId":"evt_...","ok":true}

$ beam activity start --key deploy --replace --style ring \
    --title "Deploy #184" --status "Building" --progress 0.1
```

## Install

```sh
go install github.com/dotbrains/beam@latest
```

## Commands

| Command | What it does |
|---|---|
| `beam serve` | Run the webhook API |
| `beam notify <body>` | Send a one-shot notification |
| `beam ask <prompt>` | Send an interactive approval, yes/no, or text prompt |
| `beam activity start` | Start progress state |
| `beam activity update <id-or-key>` | Patch progress state |
| `beam activity end <id-or-key>` | End progress state |
| `beam activity get <id-or-key>` | Read progress state |
| `beam config init` | Write local CLI config |

## Docs

- [SPEC.md](SPEC.md) is the verbose product goal and feature map.
- [docs/architecture.md](docs/architecture.md) explains the system model.
- [docs/api.md](docs/api.md) describes webhook routes.
- [docs/cli.md](docs/cli.md) describes command behavior.
- [docs/ci.md](docs/ci.md) describes quality gates and LOC budgets.

## Development

```sh
make ci
```
