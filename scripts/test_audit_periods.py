import json
import tempfile
import unittest
from pathlib import Path
from audit_periods import audit, classify, signature

class PeriodAuditTests(unittest.TestCase):
    def test_ambiguous_and_low_full_lines_are_preserved(self):
        row = dict(game_id=1, player_id=2, book='draftkings', stat='points', line=3.5)
        key = signature(row)
        self.assertEqual(classify(row, {key: {'full'}}), 'full')
        self.assertEqual(classify(row, {key: {'partial', 'full'}}), 'ambiguous')
        self.assertEqual(classify(row, {}), 'unmatched')
        self.assertEqual(classify(row, {key: {'partial'}}), 'suspected')

    def test_apply_backs_up_and_quarantines_matching_partial(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root/'snapshots').mkdir()
            prop = dict(game_id=1, player_id=2, vendor='dk', prop_type='points_1q',
                        line_value='6.5', market={'type':'over_under'})
            (root/'snapshots'/'props_2026-11-10.jsonl').write_text(json.dumps({'response':{'data':[prop]}})+'\n')
            rows = [dict(game_id=1, player_id=2, book='dk', stat='points', period='1q', line=6.5),
                    dict(game_id=1, player_id=3, book='dk', stat='points', line=4.5)]
            original = ''.join(json.dumps(r)+'\n' for r in rows)
            path = root/'settled.jsonl'
            path.write_text(original)
            audit(root)
            self.assertEqual(path.read_text(), original)
            audit(root, True)
            self.assertEqual([json.loads(x) for x in path.read_text().splitlines()],rows[1:])
            self.assertEqual(next(root.glob('*.bak')).read_text(),original)
            self.assertEqual(len(list(root.glob('*.excluded'))),1)

if __name__ == '__main__':
    unittest.main()
