import os
import tempfile
import unittest

from calltoverify_pi.retry import RetryQueue


class RetryTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp()
        self.path = os.path.join(self.dir, "q.jsonl")

    def test_add_and_flush_all(self):
        q = RetryQueue(self.path)
        q.add({"n": 1})
        q.add({"n": 2})
        self.assertEqual(len(q), 2)

        sent = []
        n = q.flush(sent.append)
        self.assertEqual(n, 2)
        self.assertEqual(len(q), 0)
        self.assertEqual(sent, [{"n": 1}, {"n": 2}])

    def test_partial_flush_preserves_order(self):
        q = RetryQueue(self.path)
        q.add({"n": 1})
        q.add({"n": 2})
        q.add({"n": 3})

        seen = []

        def send(item):
            seen.append(item)
            if item["n"] == 2:
                raise RuntimeError("boom")

        n = q.flush(send)
        self.assertEqual(n, 1)  # only n=1 succeeded
        self.assertEqual(q._read(), [{"n": 2}, {"n": 3}])  # remainder kept, in order

    def test_empty_flush(self):
        q = RetryQueue(self.path)
        self.assertEqual(q.flush(lambda _: None), 0)

    def test_permanent_failure_is_dropped(self):
        # A 4xx (other than 429) means the backend rejected the item itself; it
        # must be dropped so it cannot block the queue forever.
        q = RetryQueue(self.path)
        q.add({"n": 1})
        q.add({"n": 2})
        q.add({"n": 3})

        class Rejected(Exception):
            status = 400

        seen = []

        def send(item):
            seen.append(item)
            if item["n"] == 2:
                raise Rejected()

        n = q.flush(send)
        self.assertEqual(n, 2)  # n=1 and n=3 sent
        self.assertEqual(seen, [{"n": 1}, {"n": 2}, {"n": 3}])  # all attempted
        self.assertEqual(len(q), 0)  # poison item dropped, queue drained

    def test_transient_429_is_kept(self):
        q = RetryQueue(self.path)
        q.add({"n": 1})
        q.add({"n": 2})

        class Busy(Exception):
            status = 429

        def send(item):
            if item["n"] == 1:
                raise Busy()

        n = q.flush(send)
        self.assertEqual(n, 0)
        self.assertEqual(q._read(), [{"n": 1}, {"n": 2}])  # 429 is transient -> kept

    def test_skips_malformed_line(self):
        with open(self.path, "w", encoding="utf-8") as fh:
            fh.write('{"n": 1}\n')
            fh.write("not json\n")
            fh.write('{"n": 2}\n')
        q = RetryQueue(self.path)
        self.assertEqual(q._read(), [{"n": 1}, {"n": 2}])

    def test_bounded_size_drops_oldest(self):
        q = RetryQueue(self.path, max_items=2)
        q.add({"n": 1})
        q.add({"n": 2})
        q.add({"n": 3})
        self.assertEqual(q._read(), [{"n": 2}, {"n": 3}])


if __name__ == "__main__":
    unittest.main()
