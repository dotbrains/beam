# Getting started

```sh
go install github.com/dotbrains/beam@latest
beam config init
beam serve
```

`beam serve` uses SQLite by default at `~/.local/state/beam/beam.db`. For a
throwaway server, run:

```sh
beam serve --storage memory
```

In another terminal:

```sh
beam notify "hello from Beam" --title local
```

The local development server registers `dev_token` and one synthetic device so
requests can exercise the full event path without a mobile app.

`beam serve` uses the deterministic local push provider by default. To hand
delivery to an external APNs, Expo, or custom worker, run the HTTP provider
adapter:

```sh
beam serve --provider http \
  --provider-url https://push-worker.example.com/beam/deliver \
  --provider-token "$BEAM_PROVIDER_TOKEN"
```
