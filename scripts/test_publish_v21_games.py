import json
import tempfile
import unittest
from pathlib import Path
from publish_v21_games import sanitize, publish


def sample():
    return {
        "policy": "game-pace-v1",
        "at": "2026-11-21T21:05:00Z", "date": "2026-11-21",
        "games": [{"id": 33, "home": "SECRET GAME", "price": -135}],
        "evaluation": {
            market: {
                "N": 9, "Dates": 1, "CalibratedN": 0,
                "RawLogLoss": .69, "BookLogLoss": .68,
                "CalibratedLogLoss": None, "CalibrationRawLogLoss": None,
                "CalibrationBookLogLoss": None,
                "individualPicks": [{"player": "SECRET PLAYER"}]
            } for market in ("moneyline", "spread", "total")
        },
    }


class GameMarketPublisherTests(unittest.TestCase):
    def test_allowlist_exposes_no_single_game_or_player(self):
        report = sanitize(sample())
        self.assertEqual([x["id"] for x in report["markets"]], ["moneyline", "spread", "total"])
        payload = json.dumps(report)
        for forbidden in ("SECRET", "games", "individualPicks", '"price"', '"player"', "333"):
            self.assertNotIn(forbidden, payload)
        self.assertTrue(report["researchOnly"])
        self.assertEqual(report["markets"][0]["days"], 1)

    def test_rejects_invalid_policy_or_market(self):
        source = sample()
        source["policy"] = "unknown"
        with self.assertRaises(ValueError): sanitize(source)
        source = sample()
        del source["evaluation"]["total"]
        with self.assertRaises(ValueError): sanitize(source)

    def test_rejects_calibration_without_matched_scores(self):
        source = sample()
        source["evaluation"]["moneyline"]["CalibratedN"] = 1
        with self.assertRaises(ValueError): sanitize(source)
        source = sample()
        source["evaluation"]["total"]["RawLogLoss"] = float("nan")
        with self.assertRaises(ValueError): sanitize(source)

    def test_file_output_contains_aggregates_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            inp = Path(tmp)/"data.json"
            out = Path(tmp)/"public"/"games.json"
            inp.write_text(json.dumps(sample()))
            r = publish(inp, out)
            self.assertEqual(r["markets"][1]["n"], 9)
            self.assertTrue(out.exists())
            self.assertNotIn("SECRET", out.read_text())


if __name__ == "__main__":
    unittest.main()
