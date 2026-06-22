# Self-hosting

CallToVerify is built to run on your own infrastructure so phone numbers never leave it.

> Alpha: treat these as the intended operational model, not a hardened production runbook yet.

## Topology

- **Coordinator** — one stateless Go service (scale horizontally behind a load balancer).
- **Postgres** — durable state. Back it up.
- **Redis** — hot session, queue, and rate-limit state.
- **Receivers** — one or more spare Android phones and/or Raspberry Pi + GSM modems, each
  holding a SIM, reaching the Coordinator over HTTPS.

## Number pool sizing

- **SMS:** the on-screen code multiplexes one number across many concurrent verifications, so
  you need few SIMs for throughput. Add numbers mainly for redundancy and carrier coverage.
- **Missed call / DTMF (voice):** one call per SIM at a time. Size the pool to peak concurrent
  voice verifications and enable the queue.

## Operational hygiene

- **Keep SIMs active.** Prepaid SIMs (common in Bangladesh and elsewhere) can be recycled if
  idle. Rotate a small amount of usage through them.
- **Monitor receivers.** A receiver that goes offline causes silent verification failures.
  Alert on missed heartbeats and fail over to another number.
- **Watch carrier limits.** Very high inbound volume to a single number can trip carrier
  anti-spam. Spread load across the pool.

## Configuration

The Coordinator is configured entirely via `CTV_*` environment variables. Keep `api_secret`,
`device_secret`, and `webhook_secret` values out of version control; store them in your secret
manager.

## Upgrades

Schema changes ship as ordered files in `coordinator/migrations`. Apply them in order. The
`docker compose` setup applies them automatically only on a fresh database; for existing
databases, run them with your migration tool of choice.
