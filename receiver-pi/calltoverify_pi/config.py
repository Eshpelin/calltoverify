"""Receiver configuration: endpoint + device credentials + the receiver's own
MSISDN. Read from a JSON file (written by `ctv-pi pair`) with environment overrides
(CTV_ENDPOINT, CTV_DEVICE_ID, CTV_DEVICE_SECRET, CTV_MSISDN, CTV_QUEUE, CTV_DTMF_PROMPT)."""
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
    # Asterisk sound to play on a DTMF call before collecting digits. Default
    # "beep"; set to a recorded prompt like "ctv-enter-pin" to voice-guide the
    # caller (see contrib/make-dtmf-prompt.sh).
    dtmf_prompt: str = "beep"


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
        ("dtmf_prompt", "CTV_DTMF_PROMPT"),
    ):
        v = os.environ.get(env)
        if v:
            setattr(cfg, key, v)

    if not cfg.queue_path:
        cfg.queue_path = os.path.join(os.path.dirname(p), "retry.jsonl")
    return cfg


def save(cfg: Config, path: str | None = None) -> None:
    p = path or default_path()
    d = os.path.dirname(p)
    os.makedirs(d, exist_ok=True)
    try:
        os.chmod(d, 0o700)  # makedirs mode is umask-masked; enforce owner-only
    except OSError:
        pass
    # The file holds device_secret in cleartext, so keep it owner-readable only.
    fd = os.open(p, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as fh:
        json.dump(asdict(cfg), fh, indent=2)
    try:
        os.chmod(p, 0o600)  # tighten if the file pre-existed with looser perms
    except OSError:
        pass
