import unittest
from datetime import datetime
from zoneinfo import ZoneInfo
from workflow_mode import select

class ModeTests(unittest.TestCase):
    def test_dst_pair_and_queue_delay(self):
        for month, accepted, rejected in [(7,13,14),(11,14,13)]:
            # An evening queue start still resolves the intended morning cron slot.
            now=datetime(2026,month,10,22,tzinfo=ZoneInfo('America/New_York'))
            self.assertEqual(select('schedule','',f'17 {accepted} * * *','main','main',now),('daily',True))
            self.assertEqual(select('schedule','',f'17 {rejected} * * *','main','main',now),('daily',False))
    def test_manual_mode_and_branch(self):
        self.assertEqual(select('workflow_dispatch','site','','main','main'),('site',True))
        self.assertEqual(select('push','','','main','main'),('site',True))
        with self.assertRaises(ValueError):select('workflow_dispatch','site','','feature','main')

if __name__=='__main__':unittest.main()
