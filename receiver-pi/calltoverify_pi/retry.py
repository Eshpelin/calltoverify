"""A small durable FIFO retry buffer for inbound reports, so a brief network or
backend outage does not lose verifications. Backed by a JSON-lines file."""
from __future__ import annotations

import json
import os
from typing import Callable, List


class RetryQueue:
    def __init__(self, path: str) -> None:
        self.path = path

    def _read(self) -> List[dict]:
        if not os.path.exists(self.path):
            return []
        items: List[dict] = []
        with open(self.path, "r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if line:
                    items.append(json.loads(line))
        return items

    def _write(self, items: List[dict]) -> None:
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as fh:
            for it in items:
                fh.write(json.dumps(it) + "\n")
        os.replace(tmp, self.path)

    def add(self, item: dict) -> None:
        with open(self.path, "a", encoding="utf-8") as fh:
            fh.write(json.dumps(item) + "\n")

    def __len__(self) -> int:
        return len(self._read())

    def flush(self, send: Callable[[dict], None]) -> int:
        """Attempt to send each buffered item in order. Items that send without
        raising are dropped; the first failure stops the flush and keeps the
        remainder (preserving order). Returns the number of items sent."""
        items = self._read()
        sent = 0
        for i, item in enumerate(items):
            try:
                send(item)
                sent += 1
            except Exception:
                self._write(items[i:])
                return sent
        self._write([])
        return sent
