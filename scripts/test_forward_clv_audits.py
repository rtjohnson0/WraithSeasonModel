import json
import tempfile
import unittest
from pathlib import Path

from archive_forward_evidence import archive
from closing_line_audit import evaluate


class VersionArchiveTests(unittest.TestCase):
    def test_strict_picks_are_not_candidate_observations(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            data = root / "data"
            data.mkdir()
            (data / "accuracy.json").write_text(json.dumps({
                "at": "2026-11-20T19:20:05Z", "modelVersion": 6,
                "config": "pregame-test", "through": "2026-11-19",
                "accepted": 961, "slices": [{"stat": "all", "side": "ALL",
                                              "n": 18, "days": 3, "supported": False}]
            }))
            p, summary = archive(data, root / "audit")
            self.assertEqual(summary["candidate_observations"], 961)
            self.assertEqual(summary["strict_qualifying_picks"], 18)
            self.assertEqual(summary["strict_days"], 3)
            other, _ = archive(data, root / "audit")
            self.assertEqual(p, other)
            self.assertIn("/v6/pregame-test/", str(p))

    def test_distinct_versions_remain_separate(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            data = root / "data"
            data.mkdir()
            baseline = {"at": "2026-11-20T19:20:05Z", "modelVersion": 5,
                        "config": "pregame-test", "slices": [
                            {"stat": "all", "side": "ALL", "n": 4, "days": 2}]}
            (data / "accuracy.json").write_text(json.dumps(baseline))
            old, _ = archive(data, root / "audit")
            baseline["modelVersion"] = 6
            (data / "accuracy.json").write_text(json.dumps(baseline))
            new, _ = archive(data, root / "audit")
            self.assertNotEqual(old, new)
            self.assertTrue(old.exists())


class ClosingLineTests(unittest.TestCase):
    def setUp(self):
        self.folder = tempfile.TemporaryDirectory()
        self.root = Path(self.folder.name)
        self.snapshots = self.root / "snapshots"
        self.snapshots.mkdir()
        self.picks = self.root / "picklog.jsonl"
        self.pick = {
            "date": "2026-11-20", "game_id": 12, "player_id": 56,
            "player": "Example Player", "stat": "points", "side": "OVER",
            "line": 19.5, "odds": -110, "book": "sgo/fanduel",
            "logged_at": "2026-11-20T23:00:00Z",
            "quote_at": "2026-11-20T22:58:00Z",
            "tip": "2026-11-21T00:00:00Z", "period": "game"
        }
        self.snap = {
            "captured_at": "2026-11-20T23:50:00Z",
            "EventStart": "2026-11-21T00:00:00Z",
            "UpdatedAt": "2026-11-20T23:48:00Z",
            "PairVerified": True, "UnderLine": 19.5, "Period": "game",
            "Name": "Example Player", "Stat": "points", "Book": "fanduel",
            "Line": 19.5, "OverOdds": -130, "UnderOdds": 110
        }

    def tearDown(self):
        self.folder.cleanup()

    def run_audit(self):
        self.picks.write_text(json.dumps(self.pick) + "\n")
        (self.snapshots / "sgo_2026-11-20.jsonl").write_text(json.dumps(self.snap) + "\n")
        return evaluate([self.picks], self.snapshots)

    def test_matched_close_is_positive_clv(self):
        result = self.run_audit()
        self.assertEqual(result["paired_count"], 1)
        self.assertGreater(result["mean_probability_point_clv"], 0)

    def test_changed_line_is_not_equated(self):
        self.snap["Line"] = 20.5
        self.snap["UnderLine"] = 20.5
        result = self.run_audit()
        self.assertEqual(result["paired_count"], 0)
        self.assertEqual(result["coverage"]["changed_handicap_unpaired"], 1)

    def test_post_tip_and_stale_quotes_rejected(self):
        self.snap["captured_at"] = "2026-11-21T00:01:00Z"
        self.assertEqual(self.run_audit()["paired_count"], 0)
        self.snap["captured_at"] = "2026-11-20T23:50:00Z"
        self.snap["UpdatedAt"] = "2026-11-20T23:00:00Z"
        self.assertEqual(self.run_audit()["paired_count"], 0)

    def test_other_book_and_non_sgo_cannot_match(self):
        self.snap["Book"] = "draftkings"
        self.assertEqual(self.run_audit()["paired_count"], 0)
        self.snap["Book"] = "fanduel"
        self.pick["book"] = "fanduel"
        self.assertEqual(self.run_audit()["coverage"]["not_sgo_origin"], 1)


if __name__ == "__main__":
    unittest.main()
