# Systems Under Test

Reproducible experiments comparing concurrent HTTP services under controlled I/O and downstream workloads.

This repository is designed to make runtime-concurrency experiments inspectable: the services, workload contracts, container setup, load harness, tests, profiling scripts, analysis notes, and experiment requirements live together. Results are interpreted narrowly and include the controls and limitations needed to reproduce them.

## Current experiment

Experiment 001 compares:

- Java 25 virtual threads with the JDK `HttpServer` and `HttpClient`.
- Go 1.26 goroutines with the standard-library `net/http` server and client.

The workload progression is:

1. A local interruptible wait.
2. One downstream HTTP call.
3. Four-way and wider downstream fan-out.
4. Controlled downstream errors, timeouts, and capacity limits.
5. Explicit request and branch admission control.
6. Cap sweeps, runtime profiling, and delay/branch sensitivity analysis.

The current evidence is workload-specific. Java performed well for simple I/O and single downstream calls, while the tested Java HTTP-client/fan-out path reached higher CPU, memory, and tail-latency pressure sooner than Go in the one-core container configuration. This is not a general ranking of Java versus Go or virtual threads versus goroutines.

## Start here

- [Experiment 001 README](experiments/001-virtual-threads-vs-goroutines/README.md) — setup, API contract, benchmark commands, and profiling.
- [Experiment 001 requirements](docs/experiments/001-experiment-requirements.md) — scope, hypotheses, fairness controls, protocol, and acceptance criteria.
- [Experiment 001 learnings](docs/experiments/001-concurrency-learnings.md) — concepts, measured findings, and interpretation limits.
- [Experiment 001 incremental report](docs/experiments/001-incremental-report-2026-10-11.md) — cap, profiling, and sensitivity results.
- [Experiment 002 requirements](docs/experiments/002-http-client-and-scaling-requirements.md) — proposed follow-up isolating HTTP-client behavior and CPU scaling.

## Quick start

Requirements:

- Docker with Compose.
- A Git checkout of this repository.

From the Experiment 001 directory:

```sh
cd experiments/001-virtual-threads-vs-goroutines
docker compose build java go loadgen
docker compose up -d java
scripts/smoke.sh java
docker compose down
```

Run Go by replacing `java` with `go`. The pinned load generator is Vegeta 12.13.0, built with Go 1.26.0. Java uses Gradle 9.8.1 through the wrapper, and the services are built for Java 25 and Go 1.26.0.

## Repository layout

```text
docs/                     Experiment requirements, plans, and learning records
experiments/001-.../      Runnable Java and Go services and benchmark harness
  java/                   Java 25 service and tests
  go/                     Go 1.26 service and tests
  infrastructure/         Deterministic downstream dependency and healthcheck
  scripts/                Smoke tests, benchmark orchestration, profiling, analysis
  workloads/              Workload notes and load-generator image
```

Raw benchmark streams, Docker statistics, profiles, and host-specific reports are intentionally kept outside Git. Each run records source revision, image identity, tool versions, workload, timing, admission caps, and host metadata so results can be audited without treating generated artifacts as source code.

## Scope and interpretation

The benchmark uses a deterministic downstream service local to the Compose network and an open-loop offered load. `/io` is a synthetic interruptible wait; `/downstream` adds one real HTTP client path; `/fanout` adds multiple concurrent client paths. These workloads isolate useful mechanisms, but they are not production database, network, or maximum-throughput benchmarks.

The experiment uses symmetric `FANOUT_MAX_IN_FLIGHT` and `FANOUT_MAX_BRANCHES_IN_FLIGHT` controls. A 503 response caused by an exhausted cap is intentional backpressure and must be analyzed separately from transport errors, timeouts, and application failures.

## Status

Experiment 001 is complete. The next proposed increment is documented in [Experiment 002 requirements](docs/experiments/002-http-client-and-scaling-requirements.md). It would isolate Java HTTP-client variants and test one-, two-, and four-CPU configurations without changing the conclusions of Experiment 001.
