#!/usr/bin/env python3

import argparse
import json
import re
from pathlib import Path
from typing import Any


UNIT_MULTIPLIERS = {
    "b": 1,
    "kb": 1000,
    "kib": 1024,
    "mb": 1000**2,
    "mib": 1024**2,
    "gb": 1000**3,
    "gib": 1024**3,
}


def number(value: Any) -> float:
    if isinstance(value, (int, float)):
        return float(value)
    return float(str(value))


def duration_ms(value: Any) -> float:
    return number(value) / 1_000_000


def parse_bytes(value: str) -> float:
    match = re.fullmatch(r"\s*([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]+)\s*", value)
    if not match:
        return 0.0
    amount, unit = match.groups()
    return float(amount) * UNIT_MULTIPLIERS.get(unit.lower(), 1)


def parse_cpu_percent(value: str) -> float:
    return float(value.rstrip("%"))


def read_stats(path: Path) -> dict[str, Any]:
    cpu_values: list[float] = []
    memory_values: list[float] = []
    samples = 0

    for line in path.read_text(encoding="utf-8").splitlines():
        if not line.strip():
            continue
        record = json.loads(line)
        stats = record["stats"]
        cpu_values.append(parse_cpu_percent(stats["CPUPerc"]))
        memory_values.append(parse_bytes(stats["MemUsage"].split(" / ", 1)[0]))
        samples += 1

    def summary(values: list[float]) -> dict[str, float | None]:
        if not values:
            return {"min": None, "max": None, "mean": None}
        return {
            "min": round(min(values), 3),
            "max": round(max(values), 3),
            "mean": round(sum(values) / len(values), 3),
        }

    return {
        "samples": samples,
        "cpu_percent": summary(cpu_values),
        "memory_bytes": summary(memory_values),
    }


def summarize(run_dir: Path) -> dict[str, Any]:
    report = json.loads((run_dir / "report.json").read_text(encoding="utf-8"))
    metadata = json.loads((run_dir / "run.json").read_text(encoding="utf-8"))
    stats = read_stats(run_dir / "container-stats.jsonl")
    latencies = report.get("latencies", {})
    requests = int(report.get("requests", 0))
    status_codes = report.get("status_codes", {})
    successful_requests = sum(
        int(count)
        for status, count in status_codes.items()
        if str(status).startswith(("2",))
    )
    errors = report.get("errors", [])
    failed_requests = requests - successful_requests

    return {
        "run": metadata,
        "requests": requests,
        "successful_requests": successful_requests,
        "all_requests_succeeded": bool(report.get("success", False)),
        "success_ratio": (successful_requests / requests) if requests else 0.0,
        "failed_requests": failed_requests,
        "error_ratio": (failed_requests / requests) if requests else 0.0,
        "error_messages": errors,
        "throughput_requests_per_second": report.get("throughput"),
        "latency_ms": {
            "p50": duration_ms(latencies["50th"]),
            "p95": duration_ms(latencies["95th"]),
            "p99": duration_ms(latencies["99th"]),
            "mean": duration_ms(latencies["mean"]),
            "max": duration_ms(latencies["max"]),
        },
        "status_codes": status_codes,
        "container_stats": stats,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Summarize one raw Vegeta run and Docker stats stream.")
    parser.add_argument("--run-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(summarize(args.run_dir), indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
