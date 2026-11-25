import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

from verify_pregame_injuries import verify


class PregameInjuryGateTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)
        # Noon ET = 17:00Z in winter.
        self.now = datetime(2026, 11, 25, 17, 0, tzinfo=timezone.utc)
        self.date = "2026-11-25"
        self.at = (self.now - timedelta(minutes=8)).isoformat().replace("+00:00", "Z")
        self.write(self.dir / f"injury_health_{self.date}.json", {
            "date": self.date, "captured_at": self.at,
            "games": 2, "injuries_complete": True,
        })
        self.write(self.dir / f"schedule_{self.date}.json", {
            "date": self.date, "captured_at": self.at,
            "games": [
                {"ID": 1, "Home": 7, "Visitor": 16, "Date": self.date},
                {"ID": 2, "Home": 28, "Visitor": 3, "Date": self.date},
            ]
        })

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, path, value):
        path.write_text(json.dumps(value), encoding="utf-8")

    def read(self, filename):
        return json.loads((self.dir / filename).read_text(encoding="utf-8"))

    def test_complete_league_report_covers_home_and_away(self):
        out = verify(self.dir, self.now)
        self.assertEqual(out["games"], 2)
        self.assertEqual((out["home_teams"], out["away_teams"]), (2, 2))
        self.assertTrue(out["injuries_complete"])

    def test_missing_or_stale_injuries_fail_closed(self):
        file = self.dir / f"injury_health_{self.date}.json"
        file.unlink()
        with self.assertRaisesRegex(ValueError, "No successful BDL"): verify(self.dir, self.now)
        self.write(file, {"date": self.date, "captured_at": self.at, "games": 2,
                          "injuries_complete": False})
        with self.assertRaisesRegex(ValueError, "wasn't completed"): verify(self.dir, self.now)
        old = (self.now - timedelta(minutes=31)).isoformat()
        self.write(file, {"date": self.date, "captured_at": old, "games": 2,
                          "injuries_complete": True})
        with self.assertRaisesRegex(ValueError, "Stale"): verify(self.dir, self.now)

    def test_one_missing_team_fails_closed(self):
        schedule = self.read(f"schedule_{self.date}.json")
        schedule["games"][1]["Visitor"] = 0
        self.write(self.dir / f"schedule_{self.date}.json", schedule)
        with self.assertRaisesRegex(ValueError, "home or away"): verify(self.dir, self.now)

    def test_wrong_date_or_mismatched_snapshot_rejected(self):
        receipt = self.read(f"injury_health_{self.date}.json")
        receipt["date"] = "2026-11-24"
        self.write(self.dir / f"injury_health_{self.date}.json", receipt)
        with self.assertRaisesRegex(ValueError, "game date"): verify(self.dir, self.now)
        receipt["date"] = self.date
        receipt["captured_at"] = (self.now - timedelta(minutes=4)).isoformat()
        self.write(self.dir / f"injury_health_{self.date}.json", receipt)
        with self.assertRaisesRegex(ValueError, "same capture"): verify(self.dir, self.now)

    def test_complete_empty_injury_list_requires_no_fake_injured_players(self):
        # Receipt is intentionally sufficient even if the API returns zero
        # injured players; never fabricate missing statuses from missing rows.
        self.assertNotIn("player", json.dumps(verify(self.dir, self.now)))


if __name__ == "__main__":
    unittest.main()
