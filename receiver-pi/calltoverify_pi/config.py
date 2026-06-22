"""Receiver configuration: endpoint + device credentials + the receiver's own
MSISDN. Read from a JSON file (written by `ctv-pi pair`) with environment overrides
(CTV_ENDPOINT, CTV_DEVICE_ID, CTV_DEVICE_SECRET, CTV_MSISDN, CTV_QUEUE)."""
from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass


@dataclass
class Config:
    endpoint: str = ""
    device_id: str = ""
    device_secret: str = ""
    msisdn: str = ""
    queue_path: str = ""


def default_path() -> str:
    base = os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config")
    return os.path.join(base, "calltoverify", "config.json")


def load(path: str | None = None) -> Config:
    p = path or default_path()
    cfg = Config()
    if os.path.exists(p):
        with open(p, "r", encoding="utf-8") as fh:
            data = json.load(fh)
        cfg = Config(**{k: data.get(k, "") for k in asdict(Config()).keys()})

    # Environment overrides any non-empty value.
    for key, env in (
        ("endpoint", "CTV_ENDPOINT"),
        ("device_id", "CTV_DEVICE_ID"),
        ("device_secret", "CTV_DEVICE_SECRET"),
        ("msisdn", "CTV_MSISDN"),
        ("queue_path", "CTV_QUEUE"),
    ):
        v = os.environ.get(env)
        if v:
            setattr(cfg, key, v)

    if not cfg.queue_path:
        cfg.queue_path = os.path.join(os.path.dirname(p), "retry.jsonl")
    return cfg


def save(cfg: Config, path: str | None = None) -> None:
    p = path or default_path()
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, "w", encoding="utf-8") as fh:
        json.dump(asdict(cfg), fh, indent=2)
