#!/usr/bin/env python3

import argparse
import json
import statistics
from collections import defaultdict
from pathlib import Path
from typing import Any


def mean(values: list[float]) -> float | None:
    return statistics.fmean(values) if values else None


def rate_number(rate: str) -> float:
    return float(rate.removesuffix("/s"))


def aggregate_summaries(root: Path) -> list[dict[str, Any]]:
    groups: dict[tuple[str, str, str, str], list[dict[str, Any]]] = defaultdict(list)
    for path in sorted(root.glob("**/summary.json")):
        summary = json.loads(path.read_text(encoding="utf-8"))
        run = summary["run"]
        downstream = run.get("downstream", {})
        scenario = downstream.get("mode", "success")
        key = (run["service"], run["workload"], run["rate"], scenario)
        groups[key].append(summary)

    rows: list[dict[str, Any]] = []
    for (service, workload, rate, scenario), summaries in groups.items():
        rows.append(
            {
                "service": service,
                "workload": workload,
                "rate": rate,
                "rate_per_second": rate_number(rate),
                "scenario": scenario,
                "repetitions": len(summaries),
                "requests_mean": mean([float(s["requests"]) for s in summaries]),
                "throughput_mean": mean([
                    float(s["throughput_requests_per_second"])
                    for s in summaries
                    if s.get("throughput_requests_per_second") is not None
                ]),
                "success_ratio_mean": mean([float(s["success_ratio"]) for s in summaries]),
                "p50_ms_mean": mean([float(s["latency_ms"]["p50"]) for s in summaries]),
                "p95_ms_mean": mean([float(s["latency_ms"]["p95"]) for s in summaries]),
                "p99_ms_mean": mean([float(s["latency_ms"]["p99"]) for s in summaries]),
                "cpu_mean_pct": mean([
                    float(s["container_stats"]["cpu_percent"]["mean"])
                    for s in summaries
                    if s["container_stats"]["cpu_percent"]["mean"] is not None
                ]),
                "memory_max_mb_mean": mean([
                    float(s["container_stats"]["memory_bytes"]["max"]) / (1024 * 1024)
                    for s in summaries
                    if s["container_stats"]["memory_bytes"]["max"] is not None
                ]),
            }
        )
    return sorted(rows, key=lambda row: (row["workload"], row["scenario"], row["rate_per_second"], row["service"]))


def fmt(value: Any, digits: int = 2) -> str:
    if value is None:
        return "n/a"
    return f"{value:.{digits}f}"


def render_report(root: Path, rows: list[dict[str, Any]]) -> str:
    lines = [
        "# Systems Under Test: Experiment 001 Comparison",
        "",
        f"Raw result root: `{root}`",
        "",
        "This report is generated from the checked-in summarizer and the raw run artifacts at the path above. It describes this controlled run only; it is not a universal ranking of Java or Go.",
        "",
        "## Aggregated results",
        "",
        "| Service | Workload | Scenario | Rate | Reps | Throughput/s | Success | p50 ms | p95 ms | p99 ms | CPU % mean | Memory MB max |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for row in rows:
        lines.append(
            f"| {row['service']} | `{row['workload']}` | {row['scenario']} | {row['rate']} | {row['repetitions']} | {fmt(row['throughput_mean'])} | {fmt(row['success_ratio_mean'] * 100)}% | {fmt(row['p50_ms_mean'])} | {fmt(row['p95_ms_mean'])} | {fmt(row['p99_ms_mean'])} | {fmt(row['cpu_mean_pct'])} | {fmt(row['memory_max_mb_mean'])} |"
        )

    lines.extend(["", "## Observations", ""])
    comparisons: dict[tuple[str, str, str], dict[str, dict[str, Any]]] = defaultdict(dict)
    for row in rows:
        comparisons[(row["workload"], row["rate"], row["scenario"])][row["service"]] = row
    for (workload, rate, scenario), services in sorted(comparisons.items()):
        java = services.get("java")
        go = services.get("go")
        if not java or not go:
            continue
        delta = java["p99_ms_mean"] - go["p99_ms_mean"]
        direction = "higher" if delta > 0 else "lower"
        lines.append(
            f"- At `{workload}`, `{rate}`, scenario `{scenario}`, Java p99 was {fmt(abs(delta))} ms {direction} than Go in these runs; success ratios were Java {fmt(java['success_ratio_mean'] * 100)}% and Go {fmt(go['success_ratio_mean'] * 100)}%."
        )

    lines.extend(
        [
            "",
            "## Interpretation limits",
            "",
            "- The generator is open-loop and runs sequentially against one SUT at a time. Throughput is the offered/requested workload result, not proof of equal maximum capacity.",
            "- CPU and memory are Docker stats samples for the SUT container only; they are not equivalent to allocation profiles or total host cost.",
            "- `/io` is a synthetic wait, `/downstream` is one deterministic local HTTP dependency, and `/fanout` multiplies dependency traffic by its branch count. These workloads should not be generalized to PostgreSQL or production networks.",
            "- A saturation conclusion requires a sustained error/latency transition across repeated runs. A single noisy point is reported as an observation, not a threshold.",
            "",
            "## Runtime semantics inspected in code",
            "",
            "Java handles each HTTP request with a virtual-thread-per-task executor and fan-out branches with child virtual-thread tasks. Go uses the standard HTTP server goroutine per request and one child goroutine per fan-out branch. Both clients propagate cancellation, enforce timeouts, and map downstream failures to 502; the implementation-level cancellation/backpressure behavior is therefore a contract observation, not a measured performance conclusion.",
        ]
    )
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description="Aggregate Experiment 001 summaries into a cautious comparison report.")
    parser.add_argument("--results", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    rows = aggregate_summaries(args.results)
    if not rows:
        raise SystemExit(f"no summary.json files found below {args.results}")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(render_report(args.results, rows), encoding="utf-8")
    print(f"wrote {args.output} from {len(rows)} aggregate rows")


if __name__ == "__main__":
    main()
