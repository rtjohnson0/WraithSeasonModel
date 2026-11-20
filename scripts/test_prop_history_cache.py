import json
import tempfile
import unittest
from pathlib import Path

from prop_history_cache import coverage, load, merge


def record(id, data=None):
    return {"_game_id": id, "response": {"data": [] if data is None else data}}


def write(path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")


class HistoricalOddsCacheTests(unittest.TestCase):
    def test_import_idempotent_without_overwriting_existing_opening_prices(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            main = root / "data" / "opening_props_2025.jsonl"
            backfill = root / "data" / "prop_backfill" / "opening_props_2025.jsonl"
            write(main, [record(100, [{"odd": "earliest"}])])
            write(backfill, [record(100, [{"odd": "later"}]), record(101)])
            report = merge(backfill, main)
            self.assertEqual(report["added"], 1)
            self.assertEqual(report["conflicting_responses_not_overwritten"], 1)
            self.assertEqual(load(main)[0][100]["response"]["data"], [{"odd": "earliest"}])
            self.assertEqual(merge(backfill, main)["added"], 0)
            self.assertEqual(len(load(main)[0]), 2)

    def test_trailing_truncated_input_is_skipped_but_middle_corruption_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            main = Path(tmp) / "main.jsonl"
            external = Path(tmp) / "external.jsonl"
            main.write_text(json.dumps(record(1)) + "\n")
            external.write_text(json.dumps(record(2)) + "\n{incomplete")
            result = merge(external, main)
            self.assertEqual(result["added"], 1)
            self.assertEqual(result["invalid_trailing_source_rows"], 1)
            self.assertEqual(set(load(main)[0]), {1, 2})
            external.write_text("{bad\n" + json.dumps(record(3)) + "\n")
            with self.assertRaises(ValueError):
                merge(external, main)
            self.assertEqual(set(load(main)[0]), {1, 2})

    def test_response_without_actual_data_list_is_not_marked_done(self):
        with tempfile.TemporaryDirectory() as tmp:
            bad = Path(tmp) / "bad.jsonl"
            bad.write_text(json.dumps({"_game_id": 3, "response": {"error": "failed"}}) + "\n")
            with self.assertRaises(ValueError):
                load(bad)

    def test_coverage_counts_unique_games_and_empty_valid_quotes(self):
        with tempfile.TemporaryDirectory() as tmp:
            data = Path(tmp)
            write(data / "stats_2025.jsonl", [
                {"game": {"id": 1, "season": 2025}},
                {"game": {"id": 1, "season": 2025}},
                {"game": {"id": 2, "season": 2025}}
            ])
            write(data / "opening_props_2025.jsonl", [record(1), record(99)])
            status = coverage(data, 2025)
            self.assertEqual((status["cached_games"], status["required_games"]), (1, 2))
            self.assertFalse(status["ready_for_full_season_backtest"])
            self.assertEqual(status["unrecognized_cached_games"], 1)


if __name__ == "__main__":
    unittest.main()
