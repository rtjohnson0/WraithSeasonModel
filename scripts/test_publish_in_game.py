import unittest
from datetime import datetime, timezone, timedelta
from publish_in_game import accept


class PublishInGameTests(unittest.TestCase):
    def test_original_board_and_probabilities_are_never_replaced(self):
        now=datetime.now(timezone.utc).isoformat()
        board={"generatedAt":"old", "board":{"date":"2026-11-25","games":[{"plays":[{"player":"Test","line":6.5,"prob":.6}]}]}, "v2":{"board":{"games":[]}}}
        halftime={"schema":1,"date":"2026-11-25","capturedAt":now,"plays":[{"player":"Test","current":4,"remainingToOver":3}]}
        next=accept(board,halftime)
        self.assertEqual(next["board"],board["board"])
        self.assertEqual(next["v2"],board["v2"])
        self.assertEqual(next["board"]["games"][0]["plays"][0]["prob"],.6)
        self.assertEqual(next["halftime"]["plays"][0]["current"],4)
        self.assertNotIn("halftime",board)

    def test_wrong_game_date_or_stale_stats_cannot_publish(self):
        now=datetime.now(timezone.utc)
        old=(now-timedelta(minutes=11)).isoformat()
        source={"board":{"date":"2026-11-25","games":[]}}
        with self.assertRaises(ValueError):
            accept(source,{"schema":1,"date":"2026-11-24","capturedAt":now.isoformat(),"plays":[]})
        with self.assertRaises(ValueError):
            accept(source,{"schema":1,"date":"2026-11-25","capturedAt":old,"plays":[]})
        with self.assertRaises(ValueError):
            accept(source,{"schema":1,"date":"2026-11-25","capturedAt":now.isoformat(),"plays":"not a list"})


if __name__=="__main__":
    unittest.main()
