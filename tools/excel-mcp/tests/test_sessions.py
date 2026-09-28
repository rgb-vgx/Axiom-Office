import json
import os
import shutil
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from excel_mcp import bridge


class SessionsTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="sessions_")

    def tearDown(self):
        shutil.rmtree(self.dir, ignore_errors=True)

    def _write(self, name, pid, last_seen, port=1):
        path = os.path.join(self.dir, name)
        with open(path, "w", encoding="utf-8") as handle:
            json.dump({"pid": pid, "app": "wps", "family": "office", "port": port,
                       "host": "WINWORD.EXE", "lastSeenEpoch": last_seen, "document": "a.docx"}, handle)
        return path

    def test_dead_pid_is_pruned(self):
        path = self._write("dead.json", 0x7FFFFFF0, time.time())
        self.assertEqual(bridge.sessions(directory=self.dir, timeout=0.2), [])
        self.assertFalse(os.path.exists(path))

    def test_live_pid_fresh_heartbeat_kept_even_if_health_fails(self):
        # pid của chính test (còn sống), port 1 không có bridge -> healthy=False nhưng vẫn giữ (bridge bận).
        path = self._write("busy.json", os.getpid(), time.time())
        found = bridge.sessions(directory=self.dir, timeout=0.2)
        self.assertEqual(len(found), 1)
        self.assertFalse(found[0]["healthy"])
        self.assertEqual(found[0]["document"], "a.docx")
        self.assertTrue(os.path.exists(path))

    def test_stale_heartbeat_without_health_is_pruned(self):
        path = self._write("stale.json", os.getpid(), time.time() - bridge.STALE_SECONDS - 10)
        self.assertEqual(bridge.sessions(directory=self.dir, timeout=0.2), [])
        self.assertFalse(os.path.exists(path))

    def test_prune_false_keeps_files(self):
        path = self._write("dead.json", 0x7FFFFFF0, time.time())
        bridge.sessions(directory=self.dir, timeout=0.2, prune=False)
        self.assertTrue(os.path.exists(path))

    def test_es_hint_points_at_events(self):
        hint = bridge.es_hint(port=47831)
        self.assertEqual(hint["url"], "http://127.0.0.1:47831/events")
        self.assertIn("X-Auth-Token", hint["curl"])


if __name__ == "__main__":
    unittest.main()
