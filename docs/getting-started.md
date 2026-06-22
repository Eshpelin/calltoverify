# Getting started

> The project is in **alpha**. Today you can run the Coordinator and its dependencies. The
> verification endpoints are scaffolded and return `501` until Phase 1 lands. This guide will
> grow into a full end-to-end walkthrough as the receivers and SDKs come online.

## Prerequisites

- Docker (for the quick start), or Go 1.25+ to run the Coordinator directly.

## Run the stack

```bash
git clone https://github.com/Eshpelin/calltoverify.git
cd calltoverify
docker compose up --build
```

This starts Postgres, Redis, and the Coordinator. The schema migrations in
`coordinator/migrations` run automatically the first time Postgres initializes.

Check it is alive:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Run the Coordinator without Docker

```bash
cd coordinator
go run ./cmd/coordinator
```

Configure with `CTV_*` environment variables (see [`coordinator/README.md`](../coordinator/README.md)).

## What end-to-end will look like (Phase 1)

1. Create an app and get an API key.
2. Pair a receiver (the Android app) by scanning a code; it registers its number.
3. From your backend, call `startVerification()` and render the returned instructions with the
   web widget.
4. Send the code by SMS from a phone; watch the widget flip to **verified**.

Follow [`channels.md`](channels.md) to understand the options before integrating.
