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

Local credentials are stored in the config file with mode `0600`:

```sh
beam auth login --token beam_xxx \
  --scope notify \
  --scope activity \
  --client-name CI \
  --expires-in 24h
beam auth login --scope notify --client-name "Nick's Mac"
beam auth status
beam auth logout --revoke
```

Without `--token` or `BEAM_TOKEN`, `auth login` starts browser/device
authorization, prints the user code and verification URL, polls until approved,
and stores the returned token. `auth logout --revoke` revokes device-issued
credentials before clearing local config.

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
beam services devices register svc_abc --name "Nick's iPhone" \
  --push-to-start-token "$BEAM_PUSH_TO_START_TOKEN"
beam services devices list svc_abc
beam services devices deactivate svc_abc dev_123
beam services rotate-token svc_abc
beam services delete svc_abc
```

`create` and `rotate-token` print the webhook token once. `list` and `show`
return token-safe service objects. Device registration accepts Live Activity
push-to-start tokens, but device output only reports
`pushToStartTokenRegistered`.

## Prompt commands

```sh
beam ask "Deploy to production?" --approval --wait \
  --expires-in 15m --timeout 15m --poll 2s
beam notify ask "Deploy to production?" --approval --expires-in 15m
beam interaction wait evt_abc --timeout 15m --poll 2s
```

`ask --wait` and `interaction wait` return exit code `4` for timed out,
expired, or canceled prompts and `5` for denied or no responses. `--expires-in`
controls prompt expiry, while `--timeout` controls how long the CLI waits.

## Activity commands

```sh
beam activity start --key deploy --replace --style ring \
  --title "Deploy #184" --status "Building" --progress 0.1 \
  --device dev_local

beam activity update deploy --status "Testing" --progress 0.6
beam activity get deploy
beam activity list
beam activity end deploy --status "Shipped" --progress 1
```

Activity start, update, and end commands print the API JSON response before
returning exit code `7` when the response reports `accepted: 0`.

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
