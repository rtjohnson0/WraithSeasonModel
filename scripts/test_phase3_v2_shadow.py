"""Synthetic phase-3 safety tests. No proprietary pick rows or API calls."""
import hashlib
import json
import math
import tempfile
import unittest
from pathlib import Path

from phase3_v2_shadow import (assess, challenger, live_shadow, loss_summary,
                              matched_rows, replay_index, replay_key, run,
                              summarize)


class ChallengerTests(unittest.TestCase):
    def test_shadow_shrinks_disagreement_without_side_quota(self):
        self.assertAlmostEqual(challenger(0.7, 0.5, {}), 0.65)
        self.assertAlmostEqual(challenger(0.3, 0.5, {}), 0.35)
        self.assertAlmostEqual(challenger(0.7, 0.5, {"questionable": 1}), 0.61)
        self.assertAlmostEqual(challenger(0.3, 0.5, {"returning": True}), 0.39)
        self.assertAlmostEqual(challenger(0.5, 0.5, None), 0.5)
        with self.assertRaises(ValueError):
            challenger(float("nan"), 0.5, None)

    def test_loss_factors_are_descriptive_and_overlap(self):
        o = {"cohort": "verified replay", "date": "2026-11-18",
             "stat": "points", "line": 15.5, "finalOver": 0.64,
             "actual": 12, "projection": 19,
             "projectedMinutes": 32, "actualMinutes": 25,
             "context": {"confirmedOut": 1, "questionable": 1, "b2b": True,
                         "spread": 12}}
        r = {"market": 0.5, "p1": 0.64, "over_odds": -110,
             "under_odds": -110, "model_id": "a" * 64}
        row = assess({"obs": o, "quote": r})
        self.assertTrue(row["selected"])
        self.assertFalse(row["won"])
        self.assertTrue(row["indicators"]["miss_direction_minutes"])
        self.assertTrue(row["indicators"]["miss_direction_rate"])
        self.assertTrue(row["indicators"]["pregame_confirmed_teammate_out"])
        self.assertTrue(row["indicators"]["pregame_uncertain_rotation"])
        self.assertAlmostEqual(row["minute_term"] + row["rate_term"],
                               o["actual"] - o["projection"])
        self.assertNotIn("player", row)
        self.assertEqual(loss_summary([row])["loss_patterns"], {})

    def test_future_shadow_only_counts_not_recommendations(self):
        a = {"date": "2026-11-21", "at": "2026-11-20T22:30:00Z",
             "tip": "2026-11-21T00:00:00Z", "market": 0.5, "p1": 0.65,
             "audit_context": {"returning": True}}
        b = dict(a, at="2026-11-20T23:00:00Z", p1=0.8)
        key1 = ("2026-11-21", 12, 34, "points", 19.5,
                __import__("datetime").datetime.fromisoformat("2026-11-20T22:30:00+00:00"))
        key2 = ("2026-11-21", 12, 34, "points", 19.5,
                __import__("datetime").datetime.fromisoformat("2026-11-20T23:00:00+00:00"))
        report = live_shadow({key1: [a], key2: [b]}, "2026-11-20")
        self.assertEqual(report["ungraded_shadow_questions"], 1)
        self.assertAlmostEqual(report["mean_abs_probability_change"], 0.0675)
        self.assertNotIn("player", json.dumps(report).lower())
        self.assertTrue(report["not_evidence_of_results"])


class ReplayJoinTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.data = Path(self.tmp.name) / "data"
        (self.data / "replay").mkdir(parents=True)
        self.model = b'{"Version":6,"TrainedBefore":"2026-10-30"}'
        self.mid = hashlib.sha256(self.model).hexdigest()
        (self.data / "replay" / f"model_{self.mid}.json").write_bytes(self.model)
        self.records = []
        self.obs = []
        for i in range(8):
            rid = 100 + i
            r = {"date": "2026-11-18", "at": "2026-11-18T21:00:00Z",
                 "tip": "2026-11-19T00:00:00Z",
                 "quote_at": "2026-11-18T20:57:00Z",
                 "updated_at": "2026-11-18T20:55:00Z",
                 "calibration_before": "2026-11-01",
                 "selection_policy": "confidence", "period": "game",
                 "book_count": 3, "book": "sgo/book-a", "line": 15.5,
                 "stat": "points", "p1": 0.64, "market": 0.5,
                 "over_odds": -110, "under_odds": -110,
                 "model_id": self.mid,
                 "row": {"identity": {"GameID": 200, "PlayerID": rid}}}
            o = {"cohort": "verified replay", "date": r["date"],
                 "gameId": 200, "playerId": rid, "stat": "points",
                 "at": r["at"], "line": r["line"], "finalOver": r["p1"],
                 "actual": 12, "projection": 19, "projectedMinutes": 32,
                 "actualMinutes": 25, "context": {"confirmedOut": 1}}
            self.records.append(r)
            self.obs.append(o)

    def tearDown(self):
        self.tmp.cleanup()

    def write_replays(self):
        (self.data / "replay" / "decisions_2026-11-18.jsonl").write_text(
            "".join(json.dumps(r) + "\n" for r in self.records))

    def test_exact_join_and_loss_summary(self):
        self.write_replays()
        index = replay_index(self.data)
        joined, drops = matched_rows({"observations": self.obs}, index)
        self.assertEqual(len(joined), 8)
        self.assertEqual(sum(drops.values()), 0)
        result = summarize({"observations": self.obs}, index, today="2026-11-19")
        self.assertEqual(result["same_baseline_selected"]["n"], 8)
        self.assertEqual(result["loss_analysis"]["loss_count"], 8)
        self.assertEqual(result["loss_analysis"]["loss_patterns"]["all_losses"]["indicators"]
                         ["miss_direction_minutes"], 8)
        self.assertLess(result["same_baseline_selected"]["challenger"]["log_loss"],
                        result["same_baseline_selected"]["production"]["log_loss"])
        text = json.dumps(result).lower()
        self.assertNotIn("sgo/book-a", text)
        self.assertNotIn("playerid", text)
        self.assertNotIn("15.5", text)

    def test_wrong_quote_and_model_hash_fail_closed(self):
        self.records[0]["book"] = "sgo/book-b"
        self.records.append(dict(self.records[0], book="sgo/book-c"))
        self.write_replays()
        joined, drops = matched_rows({"observations": self.obs}, replay_index(self.data))
        self.assertEqual(len(joined), 7)
        self.assertEqual(drops["missing_or_ambiguous_original_quote"], 1)
        # Corrupt the frozen checkpoint; no unverified result is accepted.
        (self.data / "replay" / f"model_{self.mid}.json").write_bytes(b"tampered")
        self.assertEqual(replay_index(self.data), {})

    def test_experimental_and_post_tip_quote_rejected(self):
        self.records[0]["selection_policy"] = "value"
        self.records[1]["quote_at"] = "2026-11-19T00:01:00Z"
        self.write_replays()
        joined, _ = matched_rows({"observations": self.obs}, replay_index(self.data))
        self.assertEqual(len(joined), 6)

    def test_only_aggregate_report_is_written(self):
        self.write_replays()
        audit = Path(self.tmp.name) / "minutes.json"
        audit.write_text(json.dumps({"observations": self.obs}))
        out = Path(self.tmp.name) / "private" / "phase3.json"
        summary = run(self.data, audit, out)
        self.assertTrue(out.exists())
        self.assertEqual(summary["loss_analysis"]["selected_count"], 8)
        self.assertNotIn("playerId", out.read_text())


if __name__ == "__main__":
    unittest.main()
