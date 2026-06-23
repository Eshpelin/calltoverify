import os
import stat
import tempfile
import unittest

from calltoverify_pi.config import Config, load, save


class ConfigPermsTest(unittest.TestCase):
    def test_save_writes_owner_only_file(self):
        d = tempfile.mkdtemp()
        p = os.path.join(d, "nested", "config.json")
        save(Config(device_secret="s3cr3t", endpoint="https://x/v1", device_id="dev"), p)

        mode = stat.S_IMODE(os.stat(p).st_mode)
        self.assertEqual(mode, 0o600, oct(mode))  # device_secret must not be world/group readable

        cfg = load(p)
        self.assertEqual(cfg.device_secret, "s3cr3t")
        self.assertEqual(cfg.endpoint, "https://x/v1")


if __name__ == "__main__":
    unittest.main()
