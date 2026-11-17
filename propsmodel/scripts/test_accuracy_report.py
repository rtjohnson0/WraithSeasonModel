import unittest
from accuracy_report import render

class AccuracyReportTests(unittest.TestCase):
    def test_empty_forward_evidence_is_explicit_and_html_is_escaped(self):
        report = {'accepted': 0, 'excluded': {'<unsafe>': 1}, 'note': '<script>bad</script>'}
        result = render(report)
        self.assertIn('0 verified pregame questions', result)
        self.assertIn('no eligible completed dates', result)
        self.assertNotIn('<script>bad</script>', result)
        self.assertIn('&lt;script&gt;', result)
        self.assertIn('No production adjustment is applied', result)

if __name__ == '__main__':
    unittest.main()
