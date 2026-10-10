# Experiment 001 I/O Load Harness Design

## Goal

Add a reproducible load-generation and resource-measurement increment for Experiment 001 so the existing Java and Go `/io?delay=50` services can be measured under identical increasing request rates.

## Scope

This increment covers:

- A pinned Vegeta-based open-loop HTTP load generator.
- Warm-up and measurement phases.
- A rate matrix for the synthetic `/io` workload.
- Independent `docker stats` collection.
- Raw result, run metadata, and report artifacts.
- A runner that executes the same workload against either service.
- Analysis scripts that summarize measured data without making performance claims.

It does not add downstream HTTP, fan-out, PostgreSQL, pprof, JFR, or failure-recovery workloads. The existing `/io` endpoint remains a synthetic blocking-wait control, not real downstream I/O.

## Why Vegeta first

Vegeta provides an established open-loop request-rate generator with raw attack results and latency/error reporting. This keeps load-generator correctness separate from the first server comparison. A custom Go load generator remains a future learning-focused increment after the measurement protocol is stable.

## Execution model

The runner operates on one service at a time:

1. Build or select the pinned service image.
2. Start only the selected Compose service.
3. Wait for its health check.
4. Run a warm-up attack whose results are discarded.
5. Start an independent `docker stats` sampler.
6. Run the measurement attack at a configured request rate and duration.
7. Stop metrics collection and the service.
8. Store raw and derived artifacts under a unique run directory.

Java and Go receive the same URL path, request rate, duration, timeout, connection limit, container CPU limit, and memory limit. The load generator is not included in service CPU/memory measurements.

## Artifact schema

Each run is stored under `results/raw/<run-id>/<service>/`:

- `attack.bin`: raw Vegeta result stream.
- `report.json`: Vegeta JSON report generated from the raw stream.
- `container-stats.jsonl`: timestamped Docker container statistics.
- `run.json`: service, workload, rates, durations, tool versions, image identity, Git commit, and host metadata.

Validation artifacts are explicitly named and are not treated as research findings. The results directory must permit checked-in JSON/CSV artifacts when a measured run is intentionally published.

## Initial protocol

The default validation matrix uses `/io?delay=50` and rates of 10, 100, 500, 1,000, and 2,000 requests/sec. The default harness duration is short enough for validation; publication runs will override duration and repetition count through explicit configuration. Warm-up and measurement durations are always recorded separately.

Vegeta reports request latency and status/error outcomes. Docker stats provide independent CPU, memory, network, and throttling observations. No result is interpreted as a runtime advantage until repeated runs, environment metadata, and workload limitations are reviewed.

## Analysis boundaries

The analysis layer may calculate throughput, p50/p95/p99 latency, error rate, and resource summaries from raw artifacts. It must not infer causality from a single run, compare unlike warm-up states, or describe this synthetic delay workload as real downstream I/O.

