# Getting started

```sh
go install github.com/dotbrains/beam@latest
beam config init
beam serve
```

In another terminal:

```sh
beam notify "hello from Beam" --title local
```

The local development server registers `dev_token` and one synthetic device so
requests can exercise the full event path without a mobile app.
