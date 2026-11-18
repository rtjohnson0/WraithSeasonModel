import io
import json
from pathlib import Path
import tempfile
import unittest
import zipfile
from import_context_artifact import merge_archive

class MergeContextTests(unittest.TestCase):
    def test_newer_daily_capture_survives_older_background_artifact(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)/'context';root.mkdir()
            def row(stamp,value):return json.dumps({'captured_at':stamp,'row':{'id':1,'game_id':7,'player_id':3,'stats':{'potential_ast':value}}})+'\n'
            (root/'passing_2026.jsonl').write_text(row('2026-11-18T12:00:00Z',12))
            archive=Path(tmp)/'context.zip'
            with zipfile.ZipFile(archive,'w') as z:
                z.writestr('passing_2026.jsonl',row('2026-11-17T12:00:00Z',10))
                z.writestr('../outside_2026.jsonl',row('2026-11-17T12:00:00Z',10))
            self.assertEqual(merge_archive(archive,root),1)
            value=json.loads((root/'passing_2026.jsonl').read_text())['row']['stats']['potential_ast']
            self.assertEqual(value,12)
            self.assertFalse((Path(tmp)/'outside_2026.jsonl').exists())
