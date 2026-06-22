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


if __name__ == "__main__":
    unittest.main()
