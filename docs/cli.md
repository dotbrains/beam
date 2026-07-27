# CLI

Beam CLI commands are designed for scripts and coding agents.

```mermaid
flowchart LR
  cli[beam CLI] --> config[Config and env]
  config --> api[Beam API]
  api --> json[One JSON object on stdout]
  api --> err[Diagnostics on stderr]
```

## Configuration

`beam config init` writes:

```yaml
api_url: http://127.0.0.1:8080
token: dev_token
```

Target behavior adds `BEAM_API_URL` and `BEAM_TOKEN` overrides, browser auth,
scopes, client names, expiration, status, logout, and revocation.

`BEAM_API_URL` and `BEAM_TOKEN` override file config for one-off scripts:

```sh
BEAM_API_URL=https://beam.example.com BEAM_TOKEN=beam_xxx beam notify "Shipped"
```

Browser auth, scopes, client names, expiration, status, logout, and revocation
remain planned.

## Notification commands

```sh
beam notify "Build 48 passed" \
  --title CI \
  --image https://example.com/ci.png \
  --url https://ci.example.com/builds/48 \
  --device dev_iphone \
  --device dev_ipad \
  --idempotency-key build-48
```

Read the notification body from stdin when another process already owns the
message text:

```sh
git log -1 --pretty=%B | beam notify --stdin --title Git
```

## Service commands

```sh
beam services create --title CI --url https://ci.example.com
beam services list
beam services update svc_abc --title Deploys
beam services devices register svc_abc --name "Nick's iPhone"
beam services devices list svc_abc
beam services devices deactivate svc_abc dev_123
beam services rotate-token svc_abc
beam services delete svc_abc
```

`create` and `rotate-token` print the webhook token once. `list` and `show`
return token-safe service objects.

## Prompt commands

```sh
beam ask "Deploy to production?" --approval --wait --timeout 15m
```

Target behavior should also expose a `notify ask` alias, `--poll`, prompt
expiry flags, and `interaction wait <id>` for resuming a wait.

## Activity commands

```sh
beam activity start --key deploy --replace --style ring \
  --title "Deploy #184" --status "Building" --progress 0.1

beam activity update deploy --status "Testing" --progress 0.6
beam activity end deploy --status "Shipped" --progress 1
```

## Exit codes

| Code | Meaning |
|---:|---|
| 0 | success, approved, yes, or replied |
| 1 | API error |
| 2 | usage error |
| 3 | authentication or scope error |
| 4 | timed out, canceled, or expired |
| 5 | denied or no |
| 6 | network error |
| 7 | no device accepted the push |
