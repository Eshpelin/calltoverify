# Security Policy

CallToVerify is phone-number verification infrastructure. A vulnerability here can let an
attacker forge a verified identity, so we take security reports seriously.

## Reporting a vulnerability

**Do not open a public issue for security vulnerabilities.**

Please report privately via one of:

- GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  on this repository, or
- email **security@calltoverify.dev**.

Include: affected component and version/commit, a description, reproduction steps or a
proof of concept, and the impact you observed.

We aim to acknowledge within **72 hours** and to provide a remediation timeline after triage.
Please give us reasonable time to ship a fix before any public disclosure. We are happy to
credit you in the advisory unless you prefer to remain anonymous.

## Supported versions

The project is in **alpha**. Until a `1.0` release, only the `main` branch receives security
fixes.

## Threat model

Before deploying, read [`docs/security.md`](docs/security.md). Key things to understand:

- **Sender / caller-ID spoofing** is possible for sophisticated attackers, the same class of
  weakness as conventional SMS OTP. For high-assurance use, combine with another factor.
- **Code brute force** is mitigated by short codes with short TTLs, single use, per-sender
  attempt caps, and auto-blocking. Tune these for your risk tolerance.
- **Receiver trust:** inbound events are HMAC-signed with a per-device secret and replay
  protection. In self-hosted deployments you own the device, so the trust boundary is yours.

## Scope

In scope: the Coordinator, receivers, SDKs, and widget in this repository. Out of scope:
third-party carrier infrastructure, the SIMs themselves, and the security of a deployer's own
hosting environment.
