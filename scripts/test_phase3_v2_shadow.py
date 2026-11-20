"""Synthetic tests for actual V2 rotation-v4 shadow analysis."""
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from phase3_v2_shadow import (
    assess, challenger, live_shadow, loss_summary, metrics,
    read_go_verified, run, summarize, valid_question,
)


def make_question(**changes):
    original = {
        "date": "2026-11-18", "stat": "points", "model_id": "a" * 64,
        "p_over": 0.64, "market": 0.5,
        "over_odds": -110, "under_odds": -110,
        "line": 15.5, "actual": 12.0, "projected": 19.0,
        "projected_minutes": 32.0, "actual_minutes": 25.0,
        "confirmed_out": 1, "relevant_out": 1,
        "matched_games": 2, "allocation_applied": False,
        "rotation_supported": False, "own_status": "",
        "pregame_lineups": 1,
    }
    original.update(changes)
    # Go's devigged -110/-110 market is exactly 0.5.
    return original


class ChallengerTests(unittest.TestCase):
    def test_fixed_market_residual_and_no_over_quota(self):
        self.assertAlmostEqual(challenger(.7, .5, {}), .65)
        self.assertAlmostEqual(challenger(.3, .5, {}), .35)
        uncertain = {"confirmed_out": 1, "allocation_applied": False,
                     "rotation_supported": False}
        self.assertAlmostEqual(challenger(.7, .5, uncertain), .61)
        self.assertAlmostEqual(challenger(.3, .5, uncertain), .39)
        self.assertAlmostEqual(challenger(.5, .5, uncertain), .5)
        with self.assertRaises(ValueError):
            challenger(float("nan"), .5, {})

    def test_loss_attribution_is_arithmetic_not_causal(self):
        row = assess(make_question())
        self.assertTrue(row["selected"])
        self.assertFalse(row["won"])
        self.assertTrue(row["indicators"]["minutes_error_in_miss_direction"])
        self.assertTrue(row["indicators"]["rate_error_in_miss_direction"])
        self.assertTrue(row["indicators"]["pregame_relevant_teammate_out"])
        self.assertTrue(row["indicators"]["pregame_unsupported_rotation"])
        self.assertAlmostEqual(row["minute_term"] + row["rate_term"], 12 - 19)
        self.assertNotIn("player", row)
        self.assertEqual(loss_summary([row])["loss_patterns"], {})

    def test_precision_filter_only_affects_shadow_research(self):
        # A supported, reasonably stable minutes projection can qualify at 62%.
        stable = make_question(
            p_over=.7, relevant_out=1, rotation_supported=True,
            allocation_applied=True, matched_games=4,
            projected_minutes=30, actual_minutes=30, actual=22.0)
        result = assess(stable)
        self.assertTrue(result["selected"])
        self.assertTrue(result["precision_research_selected"])
        # An unsupported teammate-absence workload still remains part of the
        # baseline comparison but cannot qualify for the research precision filter.
        unsupported = make_question(p_over=.7, actual=22.0)
        result = assess(unsupported)
        self.assertTrue(result["selected"])
        self.assertFalse(result["precision_research_selected"])
        # More conservative published picks remain untouched in either case.
        self.assertTrue(assess(make_question())["selected"])
        self.assertFalse(assess(make_question())["precision_research_selected"])

    def test_outcomes_are_scored_only_for_matching_available_quotes(self):
        self.assertTrue(valid_question(make_question(), "2026-11-20"))
        self.assertFalse(valid_question(make_question(market=.7), "2026-11-20"))
        self.assertFalse(valid_question(make_question(over_odds=0), "2026-11-20"))
        self.assertFalse(valid_question(make_question(actual=15.5), "2026-11-20"))
        self.assertFalse(valid_question(make_question(actual_minutes=0), "2026-11-20"))
        self.assertFalse(valid_question(make_question(date="2026-11-20"), "2026-11-20"))

    def test_multiple_losses_grouped_but_no_individual_pick_exposed(self):
        sample = [make_question() for _ in range(8)]
        analyzed = [assess(x) for x in sample]
        report = summarize(sample, {}, {"questions_ungraded": 0})
        self.assertEqual(report["verified_questions"], 8)
        self.assertEqual(report["loss_analysis"]["loss_count"], 8)
        self.assertEqual(report["loss_analysis"]["loss_patterns"]["all_losses"]["indicators"]
                         ["minutes_error_in_miss_direction"], 8)
        self.assertEqual(report["same_fixed_rule_subset"]["n"], 8)
        self.assertLess(report["same_fixed_rule_subset"]["shadow_v2"]["log_loss"],
                        report["same_fixed_rule_subset"]["current_v2"]["log_loss"])
        payload = json.dumps(report)
        self.assertNotIn('"over_odds"', payload)
        self.assertNotIn('"line"', payload)
        self.assertNotIn('"player_id"', payload)
        self.assertNotIn('"actual"', payload)
        self.assertEqual(len(analyzed), 8)


class VerifiedInputTests(unittest.TestCase):
    def test_temporary_verified_input_removed_after_aggregation(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / "private" / "phase3_rotation_v4.jsonl"
            source.parent.mkdir()
            source.write_text("".join(json.dumps(make_question()) + "\n" for _ in range(8)))
            out = root / "run_evidence" / "phase3" / "aggregate.json"
            report = run(root / "data", source, out, "2026-11-20")
            self.assertEqual(report["verified_questions"], 8)
            self.assertTrue(out.exists())
            self.assertFalse(source.exists())
            self.assertNotIn('"line"', out.read_text())

    def test_unknown_input_yields_no_fabricated_results(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "missing.jsonl"
            rows, errors = read_go_verified(source, "2026-11-20")
            self.assertEqual(rows, [])
            self.assertEqual(errors["go_verified_private_input_missing"], 1)

    def test_future_live_only_uses_hashed_rotation_v4_checkpoint(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp) / "v2_rotation" / "context" / "replay"
            folder.mkdir(parents=True)
            model = b'{"version":6,"trainedBefore":"2026-11-01"}'
            digest = hashlib.sha256(model).hexdigest()
            (folder / f"model_{digest}.json").write_bytes(model)
            r = {
                "date": "2026-11-21",
                "at": "2026-11-20T22:30:00Z",
                "tip": "2026-11-21T00:00:00Z",
                "quote_at": "2026-11-20T22:27:00Z",
                "updated_at": "2026-11-20T22:25:00Z",
                "selection_policy": "rotation-v4", "probability_method": "linear",
                "period": "game", "model_id": digest,
                "stat": "points", "p1": 0.65, "market": 0.5,
                "row": {"identity": {"GameID": 22, "PlayerID": 33}},
                "rotation": {"confirmedOut": [], "supported": True,
                             "allocationApplied": True},
            }
            (folder / "decisions_2026-11-21.jsonl").write_text(json.dumps(r) + "\n")
            report = live_shadow(Path(tmp), "2026-11-20")
            self.assertEqual(report["questions_ungraded"], 1)
            self.assertIsNone(report["mean_absolute_probability_adjustment"])
            # Checkpoint mismatch must fail closed.
            (folder / f"model_{digest}.json").write_bytes(b"changed")
            self.assertEqual(live_shadow(Path(tmp), "2026-11-20")["questions_ungraded"], 0)


if __name__ == "__main__":
    unittest.main()
