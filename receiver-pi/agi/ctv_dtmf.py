#!/usr/bin/env python3
"""Asterisk AGI script for the DTMF channel.

Wire this from your dialplan (see contrib/asterisk-extensions.conf.example). On an
incoming call it answers, collects the digits the user types, and reports them to
the backend as an inbound 'call' event whose body carries the DTMF code.

NOTE: this is integration glue that requires a real Asterisk + GSM modem to run, so
it is not exercised by the unit tests. The signing/reporting it relies on (CtvClient)
is unit-tested.
"""
import sys

from calltoverify_pi.client import CtvClient
from calltoverify_pi.config import load


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

    _agi("ANSWER")
    # Play the prompt, then collect up to 6 digits, finished by # or a 7s timeout.
    # cfg.dtmf_prompt defaults to "beep"; set it to a recorded voice prompt like
    # "ctv-enter-pin" (CTV_DTMF_PROMPT or contrib/make-dtmf-prompt.sh) to guide the
    # caller to enter the code shown on their screen.
    prompt = cfg.dtmf_prompt or "beep"
    result = _agi(f"GET DATA {prompt} 7000 6")
    digits = ""
    if result.startswith("200"):
        # "200 result=<digits> (timeout)" or "200 result=<digits>"
        part = result.split("result=", 1)[-1].split(" ", 1)[0]
        digits = "".join(ch for ch in part if ch.isdigit())
    _agi("HANGUP")

    if cfg.msisdn and digits:
        try:
            CtvClient(cfg.endpoint, cfg.device_id, cfg.device_secret).inbound(
                cfg.msisdn, "call", caller, digits
            )
        except Exception as exc:  # noqa: BLE001
            sys.stderr.write(f"ctv dtmf report failed: {exc}\n")


if __name__ == "__main__":
    main()
