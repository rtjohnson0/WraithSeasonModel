import unittest
from datetime import datetime, timezone
from zoneinfo import ZoneInfo

from minutes_context_diagnostic import diagnose
from near_tip_window import in_window


def row(cohort, game, player, stat, at, projected, actual, context=None):
    return {
        "cohort": cohort, "date": "2026-11-19", "gameId": game,
        "playerId": player, "stat": stat, "at": at,
        "projectedMinutes": projected, "actualMinutes": actual,
        "context": context,
    }


class MinutesContextDiagnosticTests(unittest.TestCase):
    def test_player_game_not_multiplied_by_props(self):
        context = {"confirmedOut": 2, "assumedOut": 0, "questionable": 1,
                   "doubtful": 0, "returning": False, "b2b": False}
        report = {"observations": [
            row("verified replay", 10, 50, "points", "2026-11-19T17:00:00Z", 20, 27, context),
            row("verified replay", 10, 50, "assists", "2026-11-19T17:02:00Z", 20, 27, context),
            row("legacy projection diagnostic", 10, 50, "points", "2026-11-19T17:00:00Z", 20, 19)
        ]}
        result = diagnose(report)
        groups = {(g["cohort"], g["group"]): g for g in result["groups"]}
        strict = groups[("verified replay", "all player-games")]
        self.assertEqual(strict["playerGames"], 1)
        self.assertEqual(strict["actualMinusProjectedMinutes"], 7)
        self.assertEqual(groups[("verified replay", "confirmed teammate absent")]["playerGames"], 1)
        self.assertEqual(groups[("verified replay", "uncertain teammate status")]["playerGames"], 1)
        self.assertEqual(groups[("legacy projection diagnostic", "context unavailable")]["playerGames"], 1)
        self.assertTrue(strict["insufficientEvidence"])

    def test_later_forecast_not_substituted(self):
        observations = [
            row("verified replay", 10, 50, "points", "2026-11-19T19:00:00Z", 30, 25),
            row("verified replay", 10, 50, "rebounds", "2026-11-19T17:00:00Z", 20, 25),
        ]
        result = diagnose({"observations": observations})
        self.assertEqual(result["groups"][0]["actualMinusProjectedMinutes"], 5)

    def test_no_false_context_from_missing(self):
        result = diagnose({"observations": [
            row("verified replay", 10, 50, "points", "2026-11-19T17:00:00Z", 20, 25, None)
        ]})
        labels = {g["group"] for g in result["groups"]}
        self.assertIn("context unavailable", labels)
        self.assertNotIn("no confirmed teammates absent", labels)


class QuoteScheduleTests(unittest.TestCase):
    def test_eastern_daylight_savings(self):
        ny = ZoneInfo("America/New_York")
        self.assertTrue(in_window(datetime(2026, 7, 1, 18, 0, tzinfo=ny)))
        self.assertTrue(in_window(datetime(2026, 11, 20, 23, 30, tzinfo=ny)))
        self.assertFalse(in_window(datetime(2026, 11, 20, 13, 0, tzinfo=ny)))
        self.assertFalse(in_window(datetime(2026, 11, 20, 2, 0, tzinfo=ny)))


if __name__ == "__main__":
    unittest.main()
