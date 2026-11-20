import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from refresh_models import refresh, cached_games, parse_holdout
from calibrate_games import valid_calibration


class RefreshTests(unittest.TestCase):
    def test_full_history_evaluates_candidate_without_touching_live_pair(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd()
            os.chdir(tmp)
            try:
                Path("data").mkdir()
                orig_model='{"version":6,"trained_before":"2026-11-17"}'
                orig_cal='{"model_version":6,"trained_before":"2026-06-13"}'
                Path("data/model.json").write_text(orig_model)
                Path("data/calibration.json").write_text(orig_cal)
                research={"passes_exploratory_historical_floor":True}
                with patch("refresh_models.cached_games",return_value=(1315,1315)), \
                     patch("refresh_models.subprocess.run"), \
                     patch("refresh_models.research_candidate",return_value=research) as candidate:
                    self.assertTrue(refresh(2026))
                    candidate.assert_called_once_with(2026,False)
                self.assertEqual(Path("data/model.json").read_text(),orig_model)
                self.assertEqual(Path("data/calibration.json").read_text(),orig_cal)
                status=json.loads(Path("data/model_refresh.json").read_text())
                self.assertTrue(status["productionModelUnchanged"])
                self.assertEqual(status["status"],"candidate_evaluated")
            finally:
                os.chdir(old)

    def test_candidate_failure_never_replaces_live_model_or_blocks_publishing(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd()
            os.chdir(tmp)
            try:
                Path("data").mkdir()
                Path("data/model.json").write_text('{"version":6}')
                Path("data/calibration.json").write_text('{"model_version":6}')
                with patch("refresh_models.cached_games",return_value=(1315,1315)), \
                     patch("refresh_models.subprocess.run"), \
                     patch("refresh_models.research_candidate",side_effect=RuntimeError("unavailable")):
                    self.assertFalse(refresh(2026))
                self.assertEqual(json.loads(Path("data/model.json").read_text())["version"],6)
                self.assertEqual(json.loads(Path("data/calibration.json").read_text())["model_version"],6)
                self.assertEqual(json.loads(Path("data/model_refresh.json").read_text())["status"],
                                 "candidate_evaluation_failed")
            finally:
                os.chdir(old)

    def test_partial_history_keeps_pair_and_does_not_train(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd()
            os.chdir(tmp)
            try:
                Path("data").mkdir()
                Path("data/model.json").write_text('{"version":6}')
                Path("data/calibration.json").write_text('{"model_version":6}')
                with patch("refresh_models.cached_games",return_value=(130,1315)), \
                     patch("refresh_models.subprocess.run") as run, \
                     patch("refresh_models.research_candidate") as candidate:
                    self.assertFalse(refresh(2026))
                    self.assertEqual(run.call_count,1)
                    candidate.assert_not_called()
                self.assertIn("-maxminutes",run.call_args.args[0])
                self.assertEqual(json.loads(Path("data/model_refresh.json").read_text())["cachedGames"],130)
                self.assertEqual(json.loads(Path("data/model.json").read_text())["version"],6)
            finally:
                os.chdir(old)

    def test_partial_history_without_pair_stops(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd()
            os.chdir(tmp)
            try:
                Path("data").mkdir()
                with patch("refresh_models.cached_games",return_value=(130,1315)), \
                     patch("refresh_models.subprocess.run"):
                    with self.assertRaises(RuntimeError):
                        refresh(2026)
            finally:
                os.chdir(old)

    def test_game_policy_and_cutoff_are_required(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/"cal.json"
            cal=dict(policy="game-v2",trained_before="2026-06-13",
                     margin=dict(sigma=14),total=dict(sigma=18))
            p.write_text(json.dumps(cal))
            self.assertTrue(valid_calibration(p,"2026-11-17"))
            cal["trained_before"]="2026-11-17"
            p.write_text(json.dumps(cal))
            self.assertFalse(valid_calibration(p,"2026-11-17"))

    def test_holdout_requires_market_baseline_score(self):
        sample="""Calibration is fitted on the first half and everything below is judged on the second half
(731 lines the calibration never saw).
What the model adds beyond the sportsbook's price alone: +0.0012 log loss per line, z = +2.1
"""
        self.assertEqual(parse_holdout(sample),{
            "held_out_questions":731,
            "gain_over_market_log_loss":.0012,
            "date_clustered_z":2.1
        })
        self.assertIsNone(parse_holdout("Only 10 lines could be scored"))


class BackfillCoverageTests(unittest.TestCase):
    def test_count_ignores_duplicate_foreign_and_incomplete_rows(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd()
            os.chdir(tmp)
            try:
                Path("data").mkdir()
                Path("data/stats_2025.jsonl").write_text("\n".join(json.dumps({
                    "game":{"id":i,"season":2025}}) for i in [1,1,2]))
                Path("data/opening_props_2025.jsonl").write_text("\n".join([
                    json.dumps({"_game_id":1,"response":{"data":[]}}),
                    json.dumps({"_game_id":1,"response":{"data":[]}}),
                    json.dumps({"_game_id":99,"response":{"data":[]}}),
                    "{broken"]))
                self.assertEqual(cached_games(2025),(1,2))
            finally:
                os.chdir(old)


if __name__=="__main__":
    unittest.main()
