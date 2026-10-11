# Experiment 001 Java Virtual Threads and Go Goroutines Requirements

## Purpose

This document defines the requirements for Experiment 001, which compares a Java 25 HTTP service using virtual threads with a Go 1.26 HTTP service using goroutines under increasing I/O concurrency.

The experiment is intended to produce a reproducible, technically cautious comparison that can support a later engineering blog. The blog must explain the workload, controls, measurements, findings, and limits. It must not present the experiment as a universal ranking of Java and Go or of virtual threads and goroutines.

## Primary question

Under the same containerized HTTP workload and open-loop offered load, how do Java virtual threads and Go goroutines compare when the work is:

1. A local interruptible wait.
2. One downstream HTTP call.
3. Four or more concurrent downstream HTTP calls per request.

The comparison must distinguish the cost of request scheduling from the cost of the surrounding HTTP server, HTTP client, connection management, allocation, garbage collection, network path, and downstream capacity.

## Hypotheses

The experiment begins with these testable hypotheses:

- Both runtimes should handle a local I/O wait efficiently because the request spends most of its lifetime waiting rather than consuming CPU.
- Both runtimes should benefit from shared downstream clients and connection reuse.
- Fan-out should amplify per-request coordination, connection, allocation, and response-handling costs because one incoming request creates multiple downstream calls.
- Virtual threads and goroutines reduce the cost of blocking request code, but neither removes downstream latency or makes the dependency complete work faster.
- At a fixed offered rate, longer latency increases in-flight work. With fixed admission limits, longer occupancy eventually creates capacity rejection.

These are hypotheses, not conclusions. The report must show the evidence for and against them.

## Implementations

### Java service

- Java 25.
- JDK `HttpServer`.
- Virtual-thread request execution.
- JDK `HttpClient` for downstream HTTP/1.1 calls.
- Blocking `HttpClient.send()` calls on virtual threads for the baseline fan-out path.

### Go service

- Go 1.26.0.
- Standard-library `net/http` server.
- Runtime-managed goroutine per HTTP request.
- Standard-library `net/http` client for downstream calls.

Both services must expose the same port and response contract inside their containers and use the same deterministic downstream dependency.

## API requirements

Both services must implement:

### Health

`GET /health` returns HTTP 200 and:

```json
{"status":"ok"}
```

### Local wait

`GET /io?delay=<milliseconds>` accepts an integer delay from 0 through 5000. The wait must be interruptible by request cancellation. Invalid input returns HTTP 400 with a JSON error. A valid request returns HTTP 200 with the requested delay.

### Single downstream call

`GET /downstream?delay=<milliseconds>` makes one request to the private deterministic dependency. A successful downstream response maps to HTTP 200. Downstream non-2xx responses, transport failures, and timeouts map to HTTP 502. Incoming cancellation must be propagated.

### Fan-out

`GET /fanout?delay=<milliseconds>&branches=<count>` starts one downstream call per branch and joins all branch results. Branch count must be validated and bounded. All branches must succeed for the incoming request to succeed. Any branch failure maps to HTTP 502, and cancellation must cancel child work.

The response body, status mapping, delay validation, timeout behavior, and cancellation semantics must be equivalent across implementations.

## Admission control requirements

Fan-out admission must be explicit and symmetric:

- `FANOUT_MAX_IN_FLIGHT` limits active fan-out requests.
- `FANOUT_MAX_BRANCHES_IN_FLIGHT` limits active downstream branches.

A fan-out request must reserve all of its branches before starting downstream work. If either bound cannot be acquired, it returns an explicit HTTP 503 capacity response and starts no partial branch set. All permits must be released on success, downstream error, timeout, cancellation, and shutdown.

The selected cap values must be recorded in every run’s metadata. Capacity responses must be reported separately from downstream errors and transport failures.

## Required workload matrix

The baseline matrix must include:

| Workload | Purpose |
| --- | --- |
| `/io?delay=50` | Local interruptible wait control |
| `/downstream?delay=50` | One real downstream HTTP call |
| `/fanout?delay=50&branches=1` | Fan-out path with one branch |
| `/fanout?delay=50&branches=4` | Primary fan-out stress workload |
| `/fanout?delay=50&branches=8` | Wider fan-out sensitivity |

The controlled downstream scenarios must include success, deterministic downstream error, downstream timeout, and downstream capacity exhaustion. The fan-out endpoint must be included in at least one capacity-controlled validation.

The completed experiment’s primary stress point is:

```text
/fanout?delay=50&branches=4
offered rate: 800 requests/s
```

Four branches mean that the downstream sees approximately four times the incoming request rate. Reports must state both the client-visible rate and the effective branch rate.

## Load protocol

The harness must use an open-loop request rate so that offered load is not reduced merely because the SUT becomes slow. Each measured point must specify:

- Warm-up duration.
- Measurement duration.
- Offered rate.
- Request timeout.
- Load-generator connection count.
- Repetition count.
- Workload path and scenario.
- Request and branch caps.

The load generator must run in a separate container. Warm-up requests must be discarded from measured results. Java and Go must run sequentially on the same Compose project and host conditions.

## Fairness requirements

Unless a dimension is intentionally under study, hold these constant:

- Java 25 and Go 1.26.0.
- Gradle 9.8.1 and Vegeta 12.13.0.
- Docker/Compose versions and host.
- Base images and build inputs.
- Downstream image, delay, network, and capacity behavior.
- Request timeout and connection count.
- Warm-up, duration, offered rate, and repetition count.
- Request and branch admission caps.
- Container memory and CPU limits.
- Response-body size and status/error mapping.

The experiment must not change framework, client, connection-pool, CPU, or memory behavior between languages without labeling that dimension and interpreting it separately.

## Metrics and artifacts

Every run must retain:

- Raw Vegeta attack stream.
- Vegeta report.
- Per-request status outcomes or derived status counts.
- p50, p95, p99, mean, and maximum latency.
- Successful-request ratio and throughput.
- Docker CPU and memory samples for the SUT.
- Run metadata containing source revision, image identity, tool versions, workload, rate, timing, caps, host, and container settings.

The downstream container’s resource usage must remain distinguishable from the SUT’s resource usage.

Profiling is separate from benchmark measurement because profilers add overhead.

### Java profiling

Capture Java Flight Recorder data at selected stable and degraded points. Record JVM flags and JFR settings. Inspect allocation, garbage collection, thread parking, virtual-thread pinning, executor behavior, socket activity, exceptions, and HTTP-client coordination where available.

### Go profiling

Capture CPU, heap, goroutine, and runtime-trace artifacts at equivalent selected points. Inspect scheduler, garbage collection, socket, transport, buffer, and goroutine-stack behavior.

Profiles are explanatory evidence. They are not benchmark results and must not be compared to unprofiled runs as performance measurements.

## Test-first implementation requirements

Before changing either service:

1. Add a failing test for the required behavior.
2. Implement the smallest change that makes the test pass.
3. Run focused tests.
4. Run the complete language test suite and static checks.
5. Build the pinned container image and run smoke tests.

Tests must cover:

- Endpoint validation and response contracts.
- Successful local wait, downstream, and fan-out requests.
- Downstream errors, timeouts, and cancellation.
- Request admission rejection.
- All-or-nothing branch admission.
- Permit release after success, error, timeout, cancellation, and shutdown.
- Metadata recording for both admission limits.

## Required analysis

The analysis must:

- Present the workload model before presenting results.
- Explain the difference between requests, branches, permits, concurrency, and throughput.
- Separate application failures, transport failures, timeouts, and intentional capacity rejections.
- Show success ratio, tail latency, throughput, CPU, and memory together.
- Include per-run data before aggregate summaries.
- Identify whether a result is limited by the request cap, branch cap, downstream capacity, CPU, memory, or another observed condition.
- Compare both simple I/O and real downstream HTTP before discussing fan-out.
- Use profiles to generate hypotheses and controlled runs to support causal claims.
- State uncertainty and alternative explanations.

The analysis must not claim that virtual threads are inherently slower than goroutines. The supported claim is limited to the tested Java JDK HTTP server/client fan-out implementation, Go standard-library implementation, pinned versions, container limits, workload, and host environment.

## Acceptance criteria

Experiment 001 is complete when:

- Both implementations pass their full test and static-check suites.
- The pinned images build successfully.
- Smoke tests cover all public endpoint classes.
- The workload matrix and controlled failure scenarios run reproducibly.
- Java and Go use identical declared benchmark controls.
- Raw artifacts and derived summaries are preserved outside Git.
- Each run contains sufficient metadata to reproduce or audit it.
- Docker CPU and memory samples are present and nonzero for valid runs.
- Profiles are captured separately and labeled as explanatory.
- The report documents cap sweeps and sensitivity results.
- Conclusions include interpretation limits and do not generalize beyond the tested setup.

## Reproducibility requirements

Use a fresh external result directory for each experiment session. Do not edit raw attack streams, reports, profiles, or metadata after capture. Derived summaries and reports may be regenerated from raw artifacts.

Record at minimum:

- Git revision.
- Java, Go, Gradle, Vegeta, Docker, and Compose versions.
- Image digests.
- Host platform and architecture.
- CPU and memory limits.
- Workload path and scenario.
- Offered rate and effective branch rate.
- Warm-up and measurement duration.
- Request timeout and connection count.
- Request and branch admission caps.
- Downstream URL, delay, timeout, and capacity mode.

## Interpretation boundaries

This experiment is not:

- A general language benchmark.
- A maximum-throughput proof.
- A production database or network benchmark.
- A comparison of all Java virtual-thread or Go goroutine applications.
- A benchmark of frameworks that are not part of the pinned baseline.
- Evidence that a 503 capacity response is a runtime failure.

The downstream service is deterministic and local to the Compose network. The `/io` endpoint is a synthetic interruptible wait. The `/downstream` and `/fanout` endpoints add real HTTP client and socket behavior, but remain a controlled local dependency workload.

## Source artifacts and learning record

- Source: `experiments/001-virtual-threads-vs-goroutines`
- Original report: `/tmp/sut-final-comparison.kH31xH/comparison-report.md`
- Running learnings: [`001-concurrency-learnings.md`](001-concurrency-learnings.md)
- Incremental report: [`001-incremental-report-2026-10-11.md`](001-incremental-report-2026-10-11.md)
- Original raw artifacts: `/tmp/sut-final-comparison.kH31xH`
- Cap sweep artifacts: `/tmp/sut-cap-sweep-20261011` and `/tmp/sut-branch-cap-sweep-20261011`
- Runtime profiles: `/tmp/sut-final-runtime-attribution-20261011`
- Sensitivity artifacts: `/tmp/sut-final-sensitivity-20261011`
