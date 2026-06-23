#!/usr/bin/env python3
"""Asterisk AGI script for the missed-call channel.

Wire this from your dialplan (see contrib/asterisk-extensions.conf.example). It
reads the caller id from the AGI environment, hangs up without answering (so the
caller is not charged), and reports the caller as an inbound 'call' with an empty
body.

This replaces routing a spoofable ``${CALLERID(num)}`` through a shell ``System()``
call: the caller id arrives over the AGI protocol and never touches a shell, so a
crafted/spoofed caller id cannot inject commands.

NOTE: integration glue that needs a real Asterisk + GSM modem, so it is not
exercised by the unit tests. The signing/reporting it relies on (CtvClient) is.
"""
import sys

from calltoverify_pi.client import CtvClient
from calltoverify_pi.config import load
from calltoverify_pi.retry import RetryQueue


def _read_agi_env():
    env = {}
    while True:
        line = sys.stdin.readline().strip()
        if not line:
            break
        if ":" in line:
            k, v = line.split(":", 1)
            env[k.strip()] = v.strip()
    return env


def _agi(cmd):
    sys.stdout.write(cmd + "\n")
    sys.stdout.flush()
    return sys.stdin.readline().strip()


def main():
    env = _read_agi_env()
    caller = env.get("agi_callerid", "")
    cfg = load()

    _agi("HANGUP")  # reject without answering — the caller is not charged

    if cfg.msisdn and caller and caller.lower() != "unknown":
        try:
            CtvClient(cfg.endpoint, cfg.device_id, cfg.device_secret).inbound(
                cfg.msisdn, "call", caller, ""
            )
        except Exception as exc:  # noqa: BLE001
            # Don't drop the event on a transient outage: enqueue it so the
            # `ctv-pi run` daemon drains it later.
            sys.stderr.write(f"ctv on-call report failed; queued for retry: {exc}\n")
            try:
                RetryQueue(cfg.queue_path).add(
                    {"number": cfg.msisdn, "type": "call", "sender": caller, "body": ""}
                )
            except Exception as qexc:  # noqa: BLE001
                sys.stderr.write(f"ctv on-call enqueue failed: {qexc}\n")


if __name__ == "__main__":
    main()
