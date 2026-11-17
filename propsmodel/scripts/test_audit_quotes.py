import json
import tempfile
import unittest
from pathlib import Path
from audit_quotes import audit, identity

class QuoteAuditTests(unittest.TestCase):
    def test_excludes_only_explicit_failures_retains_originals(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            rows=[dict(date='2026-11-10',player_id=1,stat='points',side='OVER',book='dk',line=6.5),
                  dict(date='2026-11-10',player_id=2,stat='points_1q',side='OVER',book='dk',line=6.5),
                  dict(date='2026-11-10',player_id=3,stat='points',side='OVER',book='dk',line=6.5,
                       tip='2026-11-10T20:00:00Z',logged_at='2026-11-10T20:01:00Z')]
            path=root/'picklog.jsonl'
            content=''.join(json.dumps(r)+'\n' for r in rows);path.write_text(content)
            report=audit(root,True)
            self.assertEqual([r['status'] for r in report],['unverified','exclude','exclude'])
            self.assertEqual(path.read_text(),content)
            excluded=[json.loads(x)['id'] for x in (root/'exclusions.jsonl').read_text().splitlines()]
            self.assertNotIn(identity(rows[0]),excluded)
            self.assertEqual(len(excluded),2)

if __name__=='__main__':unittest.main()
