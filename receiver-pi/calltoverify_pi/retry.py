"""A small durable FIFO retry buffer for inbound reports, so a brief network or
backend outage does not lose verifications. Backed by a JSON-lines file."""
from __future__ import annotations

import json
import os
import sys
from typing import Callable, List


class RetryQueue:
    def __init__(self, path: str, max_items: int = 1000) -> None:
        self.path = path
        self.max_items = max_items

    def _read(self) -> List[dict]:
        if not os.path.exists(self.path):
            return []
        items: List[dict] = []
        with open(self.path, "r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    items.append(json.loads(line))
                except json.JSONDecodeError:
                    # Skip a corrupt line (e.g. a partial write before a crash)
                    # rather than letting it fail every future flush.
                    continue
        return items

    def _write(self, items: List[dict]) -> None:
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as fh:
            for it in items:
                fh.write(json.dumps(it) + "\n")
            fh.flush()
            os.fsync(fh.fileno())  # durable before the atomic replace
        os.replace(tmp, self.path)

    def add(self, item: dict) -> None:
        # Read-trim-write so the buffer stays bounded and the file is fsync'd: an
        # outage during a flood must not grow it without bound or lose data to a
        # half-written append.
        items = self._read()
        items.append(item)
        if len(items) > self.max_items:
            dropped = len(items) - self.max_items
            items = items[-self.max_items :]
            sys.stderr.write(f"retry queue full; dropped {dropped} oldest item(s)\n")
        self._write(items)

    def __len__(self) -> int:
        return len(self._read())

    def flush(self, send: Callable[[dict], None]) -> int:
        """Attempt to send each buffered item in order. Items that send without
        raising are dropped. A *permanent* failure (a 4xx other than 429, i.e. the
        backend rejected the item itself) drops that item so it cannot block the
        queue forever; a *transient* failure (network, 5xx, 429) stops the flush
        and keeps that item and the rest for the next attempt. Returns the number
        of items successfully sent."""
        items = self._read()
        sent = 0
        for i, item in enumerate(items):
            try:
                send(item)
                sent += 1
            except Exception as exc:  # noqa: BLE001 - classify, don't crash the daemon
                status = getattr(exc, "status", None)
                permanent = isinstance(status, int) and 400 <= status < 500 and status != 429
                if permanent:
                    sys.stderr.write(f"dropping permanently-rejected item (status {status}): {exc}\n")
                    continue
                # Transient: keep this item and everything after it for next time.
                self._write(items[i:])
                return sent
        self._write([])
        return sent
