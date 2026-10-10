#!/usr/bin/env python3

import json
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "analysis"))
from compare_results import aggregate_summaries  # noqa: E402


def main() -> None:
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        for service, p99 in (("java", 20.0), ("go", 15.0)):
            run = root / "run" / service
            run.mkdir(parents=True)
            metadata = {
                "service": service,
                "workload": "/io?delay=50",
                "rate": "100/s",
                "downstream": {"mode": "success"},
            }
            summary = {
                "run": metadata,
                "requests": 100,
                "successful_requests": 100,
                "success_ratio": 1.0,
                "throughput_requests_per_second": 100.0,
                "latency_ms": {"p50": 10.0, "p95": 18.0, "p99": p99},
                "container_stats": {
                    "cpu_percent": {"mean": 12.0},
                    "memory_bytes": {"max": 1024.0},
                },
            }
            (run / "summary.json").write_text(json.dumps(summary), encoding="utf-8")

        rows = aggregate_summaries(root)
        assert len(rows) == 2
        assert rows[0]["repetitions"] == 1
        assert rows[0]["p99_ms_mean"] in (15.0, 20.0)


if __name__ == "__main__":
    main()
