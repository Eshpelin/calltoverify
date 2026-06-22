# Security and threat model

CallToVerify verifies control of a phone number. Be honest about what that does and does not
prove, and tune the controls to your risk tolerance. To report a vulnerability, see
[`../SECURITY.md`](../SECURITY.md).

## What a successful verification proves

That someone could send an inbound signal (SMS, call, or DTMF) from the verified number to your
receiver, with knowledge of a short-lived code shown on the screen. This is the same assurance
class as conventional SMS OTP, just with the cost and direction reversed.

## Threats and mitigations

### Code brute force
An attacker sends random codes to a number hoping to hit a live session.
- 6-digit codes, short TTL (60–90s), single use.
- Cap active sessions per number; cap attempts per sender; auto-block on abuse.

### Sender / caller-ID spoofing
A sophisticated attacker can spoof an SMS sender or caller ID (for example, via VoIP or SS7).
- This is inherent to telephony and affects classic SMS OTP too.
- For high-assurance use cases, combine CallToVerify with a second factor.
- `claim` mode does not defeat spoofing by itself; it only checks the sender equals a claimed
  number, which a spoofer can also set.

### Rogue or compromised receiver
A receiver asserts "I received X from Y." A malicious receiver could forge that.
- Every inbound event is HMAC-signed with a per-device secret, plus a nonce and timestamp to
  prevent replay.
- Android receivers can attach a Play Integrity attestation.
- In self-hosted deployments you own the device, so this trust boundary is internal. It matters
  most if devices are ever pooled across tenants.

### Denial of service
Because the user now bears the call/SMS cost, the economics flip onto the attacker. Still:
- Protect each SIM with multi-SIM pooling and auto-blocking of flooding senders.
- Rate-limit verification creation per app and per source IP.

### Privacy
- Phone numbers are sensitive. Hash MSISDNs at rest where you do not need the plaintext.
- Self-hosting keeps all numbers on your own infrastructure.

## Transport and secrets

- All Coordinator traffic is HTTPS.
- `api_secret`, `device_secret`, and `webhook_secret` are stored as hashes server-side and kept
  in a secret manager, never in version control.
- Verify webhook signatures in your backend before trusting a `verification.verified` event.
