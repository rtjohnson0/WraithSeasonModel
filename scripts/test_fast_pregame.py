import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class FastPregamePathTests(unittest.TestCase):
    def test_no_slow_downloads_or_retraining_in_fast_run(self):
        script = (ROOT / "scripts/fast_pregame.sh").read_text(encoding="utf-8")
        for forbidden in ("cmd/pull stats", "cmd/pull advanced",
                          "cmd/pull context -seasons", "refresh_models.py",
                          "calibrate_games.py", "cmd/rotationaudit",
                          "cmd/minutesaudit", "cmd/accuracy",
                          "cmd/pull props", "cmd/pull gameodds"):
            self.assertNotIn(forbidden, script)
        self.assertIn("data/stats_", script)
        self.assertIn("go run ./cmd/pull snapshot", script)

    def test_complete_injuries_must_be_checked_before_picks(self):
        script = (ROOT / "scripts/fast_pregame.sh").read_text(encoding="utf-8")
        self.assertLess(script.index("go run ./cmd/pull snapshot"),
                        script.index("python3 scripts/verify_pregame_injuries.py"))
        self.assertLess(script.index("verify_pregame_injuries.py"),
                        script.index("go run ./cmd/predict"))
        self.assertLess(script.index("verify_pregame_injuries.py"),
                        script.index("go run ./cmd/games"))
        self.assertIn("set -euo pipefail", script)

    def test_manual_fast_and_scheduled_pretip_share_fast_route(self):
        workflow = (ROOT / ".github/workflows/daily.yml").read_text(encoding="utf-8")
        self.assertIn("default: fast", workflow)
        self.assertIn("steps.mode.outputs.mode == 'fast'", workflow)
        self.assertIn("steps.mode.outputs.mode == 'pretip'", workflow)
        self.assertGreaterEqual(workflow.count("run: bash scripts/fast_pregame.sh"), 2)
        self.assertIn("steps.mode.outputs.mode == 'daily'", workflow)


if __name__ == "__main__":
    unittest.main()
