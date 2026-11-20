#!/usr/bin/env python3
"""Read-only, versioned archive of the existing strict forward audit.

Does not reinterpret accepted observations or combine results across checkpoints.
Outputs only aggregate statistics to run_evidence (never site/public/data).
"""
import argparse
import hashlib
import json
import re
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


def safe(value):
    return re.sub(r"[^A-Za-z0-9._-]", "_", str(value))[:100]


def iter_jsonl(path):
    with path.open(encoding="utf-8") as handle:
        for line in handle:
            if line.strip():
                yield json.loads(line)


def checkpoint_inventory(data):
    """Inventory *record counts*, not verified results; do not score old configs."""
    grouped = Counter()
    checkpoints = {}
    for path in sorted((data / "replay").glob("decisions_*.jsonl")):
        for row in iter_jsonl(path):
            model_id = row.get("model_id", "")
            if not re.fullmatch("[0-9a-f]{64}", model_id):
                grouped[("unknown", "invalid_checkpoint")] += 1
                continue
            if model_id not in checkpoints:
                model_path = data / "replay" / ("model_" + model_id + ".json")
                if not model_path.exists() or hashlib.sha256(model_path.read_bytes()).hexdigest() != model_id:
                    checkpoints[model_id] = None
                else:
                    model = json.loads(model_path.read_text(encoding="utf-8"))
                    checkpoints[model_id] = model.get("version")
            grouped[(model_id, str(checkpoints[model_id]) if checkpoints[model_id] is not None else "missing_checkpoint")] += 1
    return [{"model_id": k[0], "model_version": k[1], "decision_rows_not_verified": n}
            for k, n in sorted(grouped.items())]


def archive(data, output):
    src = data / "accuracy.json"
    raw = src.read_bytes()
    report = json.loads(raw)
    version = report.get("modelVersion")
    config = report.get("config")
    generated = report.get("at")
    if not isinstance(version, int) or not config or not generated:
        raise ValueError("accuracy report missing version/config/timestamp")
    timestamp = datetime.fromisoformat(generated.replace("Z", "+00:00"))
    if timestamp.tzinfo is None:
        raise ValueError("accuracy timestamp must have a timezone")
    slices = report.get("slices", [])
    all_slice = next((s for s in slices if s.get("stat") == "all" and s.get("side") == "ALL"), None)
    if all_slice is None:
        raise ValueError("accuracy report missing strict all/ALL slice")
    digest = hashlib.sha256(raw).hexdigest()
    document = {
        "schema": 1,
        "cohort": "strict_active_model_only",
        "model_version": version,
        "config": config,
        "evaluated_through": report.get("through"),
        "generated_at": generated,
        "source_sha256": digest,
        "candidate_observations": report.get("accepted"),
        "strict_qualifying_picks": all_slice.get("n", 0),
        "strict_days": all_slice.get("days", 0),
        "supported": bool(all_slice.get("supported", False)),
        "strict_slices": slices,
        "excluded": report.get("excluded", {}),
        "checkpoint_inventory": checkpoint_inventory(data),
        "note": "Only strict slices are evaluated selections. Candidate observations are not picks. Historical replay inventory is NOT independently verified performance; no cohorts are merged."
    }
    destination = output / ("v" + str(version)) / safe(config)
    destination.mkdir(parents=True, exist_ok=True)
    name = timestamp.astimezone(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "_" + digest[:12] + ".json"
    path = destination / name
    if path.exists():
        if json.loads(path.read_text(encoding="utf-8")) != document:
            raise ValueError("refusing to overwrite changed evidence")
    else:
        with path.open("x", encoding="utf-8") as handle:
            json.dump(document, handle, indent=2, allow_nan=False)
            handle.write("\n")
    return path, document


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--out", type=Path, default=Path("run_evidence/forward_versions"))
    args = parser.parse_args()
    path, report = archive(args.data, args.out)
    print(f"Versioned strict evidence: {report['strict_qualifying_picks']} picks on "
          f"{report['strict_days']} days; archived {path}")


if __name__ == "__main__":
    main()
