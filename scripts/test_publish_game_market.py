import unittest
from datetime import datetime,timezone,timedelta
from publish_game_market import overlay

now=datetime(2026,11,27,20,0,tzinfo=timezone.utc)
def at(delta=0): return (now+timedelta(minutes=delta)).isoformat()
def fixtures():
    board={"board":{"date":"2026-11-27","games":[{"plays":[{"line":6.5,"prob":.6}]}]},"v2":{"board":{"games":[{"plays":[{"line":4.5}]}]}},"gameRecord":[{"id":1}],"generatedAt":"old"}
    quote={"quoteAt":at(-1),"sourceUpdatedAt":at(-2),"market":"spread","sides":[{"odds":-110},{"odds":-110}]}
    game={"date":"2026-11-27","at":at(-1),"games":[{"id":1,"tip":at(120),"quotes":[quote]}]}
    challenger={"date":"2026-11-27","at":at(-1),"games":[{"id":1,"quotes":[quote]}]}
    return board,game,challenger

class PublishGameMarketTest(unittest.TestCase):
 def test_overlay_preserves_all_original_picks(self):
    b,g,c=fixtures()
    next=overlay(b,g,c,now)
    self.assertEqual(next["board"],b["board"])
    self.assertEqual(next["v2"],b["v2"])
    self.assertEqual(next["gameRecord"],b["gameRecord"])
    self.assertEqual(next["gameMarkets"],g)
    self.assertEqual(next["gameChallenger"],c)
    self.assertNotIn("gameMarkets",b)
 def test_reject_stale_missing_started_and_mismatched(self):
    b,g,c=fixtures()
    for field in ("game", "challenger"):
     bad=dict(g if field=="game" else c)
     bad["at"]=at(-35)
     with self.assertRaises(ValueError): overlay(b,bad if field=="game" else g,bad if field=="challenger" else c,now)
    g["games"][0]["tip"]=at(-2)
    with self.assertRaises(ValueError): overlay(b,g,c,now)
    b,g,c=fixtures()
    g["games"][0]["quotes"]=[]
    with self.assertRaises(ValueError): overlay(b,g,c,now)
    b,g,c=fixtures()
    c["date"]="2026-11-26"
    with self.assertRaises(ValueError): overlay(b,g,c,now)

if __name__=="__main__":unittest.main()
