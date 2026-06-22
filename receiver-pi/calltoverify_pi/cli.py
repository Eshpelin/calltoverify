"""ctv-pi command line: pair, register, run (heartbeat daemon), and the
on-sms / on-call hooks invoked by gammu-smsd and Asterisk."""
from __future__ import annotations

import argparse
import json
import os
import sys
import time

from .client import CtvClient, CtvError
from .config import load, save
from .retry import RetryQueue


def _client(cfg) -> CtvClient:
    return CtvClient(cfg.endpoint, cfg.device_id, cfg.device_secret)


def cmd_pair(args) -> int:
    payload = json.loads(args.payload)
    cfg = load(args.config)
    cfg.endpoint = payload["endpoint"]
    cfg.device_id = payload["device_id"]
    cfg.device_secret = payload["device_secret"]
    if args.msisdn:
        cfg.msisdn = args.msisdn
    save(cfg, args.config)
    print(f"paired with {cfg.endpoint}")
    return 0


def cmd_register(args) -> int:
    cfg = load(args.config)
    resp = _client(cfg).register()
    numbers = resp.get("numbers", [])
    if not cfg.msisdn and numbers:
        cfg.msisdn = numbers[0]["msisdn"]
        save(cfg, args.config)
    print(json.dumps(resp, indent=2))
    return 0


def cmd_run(args) -> int:
    cfg = load(args.config)
    client = _client(cfg)
    queue = RetryQueue(cfg.queue_path)
    try:
        client.register()
    except CtvError as exc:
        print(f"register failed: {exc}", file=sys.stderr)
    while True:
        try:
            client.heartbeat()
        except CtvError as exc:
            print(f"heartbeat failed: {exc}", file=sys.stderr)
        try:
            queue.flush(lambda it: client.inbound(it["number"], it["type"], it["sender"], it.get("body", "")))
        except Exception as exc:  # noqa: BLE001 - keep the daemon alive
            print(f"retry flush error: {exc}", file=sys.stderr)
        time.sleep(args.interval)


def _report(cfg, kind: str, sender: str, body: str) -> int:
    number = cfg.msisdn
    if not number:
        print("no msisdn configured; run 'ctv-pi register' or set CTV_MSISDN", file=sys.stderr)
        return 1
    item = {"number": number, "type": kind, "sender": sender, "body": body}
    try:
        print(json.dumps(_client(cfg).inbound(number, kind, sender, body)))
    except Exception as exc:  # noqa: BLE001 - buffer for retry on any failure
        RetryQueue(cfg.queue_path).add(item)
        print(f"queued for retry: {exc}", file=sys.stderr)
    return 0


def cmd_on_sms(args) -> int:
    # gammu-smsd RunOnReceive sets SMS_1_NUMBER (sender) and SMS_1_TEXT (body).
    cfg = load(args.config)
    sender = args.sender or os.environ.get("SMS_1_NUMBER", "")
    body = args.text if args.text is not None else os.environ.get("SMS_1_TEXT", "")
    if not sender:
        print("no SMS sender (set SMS_1_NUMBER or pass --sender)", file=sys.stderr)
        return 1
    return _report(cfg, "sms", sender, body)


def cmd_on_call(args) -> int:
    # Invoked by Asterisk: missed call (no --dtmf) or DTMF code (--dtmf 4729).
    cfg = load(args.config)
    return _report(cfg, "call", args.sender, args.dtmf or "")


def main(argv=None) -> int:
    p = argparse.ArgumentParser(prog="ctv-pi", description="CallToVerify Raspberry Pi receiver")
    p.add_argument("--config", default=None, help="config file path")
    sub = p.add_subparsers(dest="cmd", required=True)

    sp = sub.add_parser("pair", help="store credentials from a pairing payload (JSON)")
    sp.add_argument("payload")
    sp.add_argument("--msisdn", default="")
    sp.set_defaults(func=cmd_pair)

    sp = sub.add_parser("register", help="register/announce this device to the backend")
    sp.set_defaults(func=cmd_register)

    sp = sub.add_parser("run", help="run the heartbeat + retry daemon")
    sp.add_argument("--interval", type=int, default=60)
    sp.set_defaults(func=cmd_run)

    sp = sub.add_parser("on-sms", help="report an inbound SMS (gammu-smsd RunOnReceive hook)")
    sp.add_argument("--sender", default="")
    sp.add_argument("--text", default=None)
    sp.set_defaults(func=cmd_on_sms)

    sp = sub.add_parser("on-call", help="report a missed call or DTMF code (Asterisk hook)")
    sp.add_argument("--sender", required=True)
    sp.add_argument("--dtmf", default="")
    sp.set_defaults(func=cmd_on_call)

    args = p.parse_args(argv)
    return args.func(args) or 0


if __name__ == "__main__":
    raise SystemExit(main())
