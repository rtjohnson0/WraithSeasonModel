"""Protect source packages from being omitted by research-output ignore rules."""
import subprocess
import tempfile
import unittest
from pathlib import Path


class SourceTracking(unittest.TestCase):
    def test_research_source_is_trackable_but_generated_runs_are_ignored(self):
        source = Path(__file__).resolve().parents[1]
        with tempfile.TemporaryDirectory() as temp:
            repo = Path(temp)
            subprocess.run(['git', 'init', '-q', str(repo)], check=True)
            (repo / '.gitignore').write_bytes((source / '.gitignore').read_bytes())
            for name, ignored in [('internal/research/study.go', False),
                                  ('internal/research/study_test.go', False),
                                  ('research/tails/run/study.json', True)]:
                path = repo / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('fixture', encoding='utf-8')
                result = subprocess.run(['git', 'check-ignore', '-q', '--', name], cwd=repo)
                self.assertEqual(result.returncode, int(not ignored), name)
