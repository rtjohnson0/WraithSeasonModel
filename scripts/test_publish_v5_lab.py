import json
import tempfile
import unittest
from pathlib import Path
from publish_v5_lab import sanitize, publish


def report():
    return {
        "Policy":"rotation-v5", "Through":"2026-11-20","Days":6,
        "PlayerGames":62,"Questions":224,"RotationMinutesMAE":4.25,
        "Phase3":[{"playerId":444,"book":"PRIVATE","line":4.5}],
        "Excluded":{"private-player-444":1},
        "Slices":[
            {"Stat":"points","N":65,"Days":6,"RotationBrier":.242,"RotationMAE":3.2,"privateLine":22.5},
            {"Stat":"threes","N":55,"Days":6,"RotationBrier":.247,"RotationMAE":1.09},
        ],
    }

class V5PublicEvidenceTests(unittest.TestCase):
    def test_allows_only_aggregates_and_separate_cohort(self):
        raw=sanitize(report())
        payload=json.dumps(raw)
        self.assertEqual(raw["policy"],"rotation-v5")
        self.assertEqual(raw["stats"][1]["stat"],"threes")
        for sensitive in ("PRIVATE","444","privateLine","private-player","22.5",'"line"'):
            self.assertNotIn(sensitive,payload)

    def test_no_fabricated_score_from_single_slate(self):
        original=report()
        original["Days"]=1
        original["Slices"][0]["Days"]=1
        original["Slices"][1]["Days"]=1
        out=sanitize(original)
        self.assertIsNone(out["minutesMAE"])
        self.assertIsNone(out["stats"][0]["brier"])

    def test_v4_cannot_be_labeled_v5(self):
        original=report()
        original["Policy"]="rotation-v4"
        with self.assertRaises(ValueError):sanitize(original)
        original=report()
        original["Slices"][0]["RotationBrier"]=float("nan")
        with self.assertRaises(ValueError):sanitize(original)

    def test_export_has_only_allowlisted_fields(self):
        with tempfile.TemporaryDirectory() as tmp:
            source=Path(tmp)/"source.json"
            dest=Path(tmp)/"public"/"v5-lab.json"
            source.write_text(json.dumps(report()))
            data=publish(source,dest)
            self.assertTrue(dest.exists())
            self.assertEqual(data["playerGames"],62)
            self.assertNotIn("PRIVATE",dest.read_text())

if __name__=="__main__":
    unittest.main()
