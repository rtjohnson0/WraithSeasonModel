import json
import tempfile
import unittest
from pathlib import Path

from publish_v21_lab import sanitize, publish


def fixture():
    return {
        "schema": 1, "policy": "v21-minute-guard-and-cap-20261121",
        "generatedAt": "2026-11-21T16:37:00Z", "through": "2026-11-20",
        "status": "insufficient_evaluation_dates",
        "dates": 3, "playerGames": 319, "questions": 1138,
        "playerName": "DO NOT EXPOSE", "book": "private",
        "candidates": [
            {"id": "base-minutes", "playerGames":319, "questions":1138, "minutesMAE":4.12,"minutesBias":.3,"brier":.249,"logLoss":.694,"statMAE":1.7,"pick": "DO NOT EXPOSE"},
            {"id": "rotation-v4", "playerGames":319, "questions":1138, "minutesMAE":4.54,"minutesBias":.2,"brier":.252,"logLoss":.705,"statMAE":1.8},
            {"id": "supported-only", "playerGames":319, "questions":1138, "minutesMAE":4.31,"minutesBias":.25,"brier":.250,"logLoss":.70,"statMAE":1.74},
            {"id": "capped-shift", "playerGames":319, "questions":1138, "minutesMAE":4.18,"minutesBias":.28,"brier":.2495,"logLoss":.698,"statMAE":1.71},
        ],
    }


class LabPublisherTests(unittest.TestCase):
    def test_public_report_is_allowlisted_no_picks_or_market_quotes(self):
        summary = sanitize(fixture())
        text = json.dumps(summary)
        for forbidden in ("DO NOT EXPOSE", '"pick"', '"book"', '"playerName"', "over_odds"):
            self.assertNotIn(forbidden, text)
        self.assertEqual(len(summary["candidates"]), 4)
        self.assertEqual(summary["candidates"][1]["id"], "rotation-v4")
        self.assertEqual(summary["candidates"][1]["brier"], .252)

    def test_unknown_candidate_rejected_instead_of_leaking_hidden_fields(self):
        original = fixture()
        original["candidates"][3]["id"] = "player-special"
        with self.assertRaises(ValueError):
            sanitize(original)

    def test_bad_nan_and_boolean_metrics_rejected(self):
        original = fixture()
        original["candidates"][2]["brier"] = float("nan")
        with self.assertRaises(ValueError):
            sanitize(original)
        original = fixture()
        original["dates"] = True
        with self.assertRaises(ValueError):
            sanitize(original)

    def test_export_to_safe_filename(self):
        with tempfile.TemporaryDirectory() as tmp:
            source, target = Path(tmp) / "data.json", Path(tmp) / "site" / "data.json"
            source.write_text(json.dumps(fixture()), encoding="utf-8")
            outcome = publish(source, target)
            self.assertEqual(json.loads(target.read_text())["dates"], 3)
            self.assertEqual(outcome["playerGames"], 319)
            self.assertNotIn("DO NOT EXPOSE", target.read_text())


if __name__ == "__main__":
    unittest.main()
