# Experiment 002 HTTP Client and Scaling Requirements

## Purpose

This document defines the requirements for the next experiment spun off from Experiment 001. The experiment must determine which parts of the observed Java fan-out degradation come from the HTTP client and connection-management path, which come from allocation and garbage collection, and which come from the one-core container boundary.

The experiment must not be framed as a general Java-versus-Go language ranking. Its subject is the tested HTTP fan-out implementation and its behavior under controlled resource and scheduling conditions.

## Background

Experiment 001 found that Java 25 virtual threads handled simple waits and a single downstream call well, but degraded sooner than Go 1.26 goroutines as concurrent downstream fan-out increased. The Java JFR showed thread parking, allocation, garbage-collection, and HTTP-client/future coordination activity, with no virtual-thread-pinned events in the captured run. Go profiles showed ordinary `net/http`, socket, scheduler, transport, buffer, and goroutine-stack work.

Admission control made the overload behavior explicit and reproducible. The current implementation uses:

- `FANOUT_MAX_IN_FLIGHT` for active fan-out requests.
- `FANOUT_MAX_BRANCHES_IN_FLIGHT` for total active downstream branches.

The new experiment should begin from the completed Experiment 001 code and retain its raw artifacts, report, and running learnings document as the baseline.

## Research questions

1. Does Java’s degradation remain when the downstream HTTP client and connection-management configuration are made more comparable to Go’s configuration?
2. Is the Java gap primarily associated with synchronous JDK `HttpClient.send()` on virtual threads, asynchronous `sendAsync()` coordination, connection reuse, response-body handling, or allocation and garbage collection?
3. How does the result change when the SUT has one, two, and four effective CPUs?
4. Which resource reaches its limit first for each implementation: CPU, heap, allocation rate, open connections, branch permits, request permits, or downstream capacity?
5. Which conclusions remain stable across repeated runs and which are sensitive to the selected cap, delay, or CPU allocation?

## Scope

The experiment must include two separately attributable tracks.

### Track A HTTP client isolation

Keep the HTTP server, request contract, downstream service, load generator, and workload shape unchanged while comparing explicitly identified client variants.

At minimum, implement and measure:

- Current Java virtual-thread fan-out using synchronous `HttpClient.send()`.
- A Java virtual-thread fan-out variant using `HttpClient.sendAsync()` with an explicitly documented completion strategy.
- Current Go `net/http` fan-out baseline.
- A Java client configuration variant with connection and transport settings documented and intentionally matched as closely as the JDK API permits.

The variant names, exact code paths, connection limits, HTTP version, timeouts, redirect policy, executor choice, and response-body consumption behavior must be recorded in run metadata. Do not call two variants equivalent unless their relevant behavior is demonstrated or clearly qualified.

If an implementation change is needed to add a variant, add it behind an explicit configuration or separate service target. Do not silently replace the Experiment 001 baseline.

### Track B CPU scaling

Repeat a selected, stable client configuration at one, two, and four effective CPUs. Keep memory limits, image versions, request caps, branch caps, downstream behavior, and offered load explicit. Do not interpret a CPU-scaled run as directly comparable to a one-CPU run without labeling the change.

Track B is intended to distinguish single-core saturation from language-runtime or HTTP-client behavior. It is not a complete machine-size benchmark.

## Required workload matrix

Use the completed Experiment 001 fan-out workload as the primary workload:

```text
/fanout?delay=50&branches=1
/fanout?delay=50&branches=4
/fanout?delay=50&branches=8
```

Use the following confirmation points after the primary matrix:

```text
/fanout?delay=10&branches=4
/fanout?delay=100&branches=4
/fanout?delay=250&branches=4
```

Run the following controls:

- `/io?delay=50` to measure scheduling and waiting without downstream HTTP.
- `/downstream?delay=50` to measure one downstream HTTP call without fan-out coordination.
- `/fanout?delay=50&branches=1` to compare the fan-out path with one branch.

The initial matrix should use 800 requests/s because that is the established stress point. Add rates of 100/s, 400/s, and 800/s for selected variants to identify whether differences appear before saturation. Do not expand the matrix until the selected points are stable across repetitions.

## Admission and capacity controls

The first comparison should use:

- `FANOUT_MAX_IN_FLIGHT=64`.
- `FANOUT_MAX_BRANCHES_IN_FLIGHT=256`.
- Downstream max in-flight set to unlimited or a documented value that is identical for both services.

The experiment must include one cap-sensitivity confirmation using branch caps 128, 256, and 512 while holding the request cap at 64. A capacity response is a measured result, not a harness failure. The analysis must distinguish successful responses, application errors, timeout errors, and intentional `503` admission responses.

## Fairness requirements

Every comparison must hold these conditions constant unless the dimension is the one being studied:

- Java 25 and Go 1.26.0 pinned versions.
- Gradle 9.8.1 and Vegeta 12.13.0.
- Same host, Docker/Compose versions, image build inputs, and dependency image.
- Same downstream delay, downstream capacity, network, request timeout, and connection count.
- Same warm-up duration, measurement duration, repetition count, and open-loop offered rate.
- Same request and branch admission caps.
- Same memory limit and explicitly recorded CPU limit.
- Same endpoint contract, response-body size, and cancellation behavior.

Run Java and Go sequentially for each point, as in Experiment 001, and use a fresh external result directory for the new experiment. Do not compare profiled runs with unprofiled benchmark runs.

## Instrumentation requirements

Each benchmark run must retain:

- Raw Vegeta attack stream and report.
- Per-run metadata including source revision, image digest, tool versions, host, CPU/memory limits, workload, rate, timeout, caps, client variant, and CPU allocation.
- Docker CPU and memory samples for the SUT and downstream service.
- HTTP status counts and latency percentiles including p50, p95, p99, mean, and maximum.
- Throughput and successful-request ratio.

Each selected profiling run must retain:

### Java

- JFR recording with at least 20 seconds of measurement after warm-up.
- JFR settings and JVM flags.
- Heap/GC information, allocation rate, thread counts, and HTTP-client connection information where available.
- Counts or evidence for virtual-thread pinning, parking, executor queueing, socket reads/writes, exceptions, and garbage collection.

### Go

- CPU profile.
- Heap profile.
- Goroutine profile.
- Runtime trace.
- Runtime and HTTP connection statistics where available.

Profiling must be run separately from benchmark measurements, and profiler overhead must be stated.

## Test-first implementation requirements

Before adding a client variant or CPU-scaling configuration:

1. Add a failing unit or integration test for the required behavior.
2. Implement the smallest change that makes the test pass.
3. Run the focused test.
4. Run the complete language test suite and static checks.
5. Build the pinned container image and run a smoke test.

Tests must cover at least:

- Successful single-branch and multi-branch fan-out.
- Downstream error and timeout mapping.
- Request cancellation and shutdown behavior.
- Full branch reservation and release on success, failure, timeout, and cancellation.
- Variant selection and metadata reporting.
- No accidental use of an unbounded executor or unbounded connection/branch admission.

## Repetition and statistical requirements

The primary matrix must use at least three repetitions per point after a short pilot confirms that the harness is stable. Report every repetition and summarize the mean, median, range, and coefficient of variation where meaningful. Do not report only the best run.

Record discarded warm-up results separately from measured results. If a run exits, loses health, produces malformed artifacts, or has zero Docker stats samples, mark it invalid and investigate it rather than silently excluding it.

## Analysis requirements

The final analysis must:

- Show raw per-run outcomes before aggregate conclusions.
- Separate capacity rejection from transport failure and application failure.
- Compare Java client variants against the current Java baseline and Go baseline.
- Relate latency and success changes to CPU, memory, allocation, GC, connection, and permit observations.
- Identify whether a result is a CPU ceiling, a branch-cap ceiling, a downstream-capacity ceiling, or an implementation failure.
- State uncertainty and alternative explanations.
- Avoid attributing causality to a profile hotspot without controlled evidence.
- Preserve the distinction between virtual-thread scheduling and the surrounding JDK HTTP-client implementation.

The primary decision should be based on repeated runs at stable points. Single-run profile evidence may explain a result, but must not establish a performance claim by itself.

## Acceptance criteria

The new experiment is complete when all of the following are true:

- All client variants and CPU configurations are explicitly named and reproducible.
- Required behavior is covered by tests before implementation changes are considered complete.
- Java and Go pass their full test and static-check suites.
- Pinned images build successfully.
- The primary matrix has at least three valid repetitions per point for both implementations.
- Raw artifacts, profiles, metadata, and derived summaries are stored outside Git under a new unique result root.
- Every summary records the request cap, branch cap, client variant, CPU allocation, and source/image identity.
- The report includes a table of per-repetition results and an explanation of invalid or excluded runs.
- Conclusions identify the strongest supported cause and explicitly list what remains unresolved.
- The report does not generalize beyond the tested HTTP client, workload, resource envelope, and versions.

## Suggested execution order

1. Copy or branch the completed Experiment 001 state without changing its artifacts.
2. Add metadata and configuration plumbing for an explicit Java client variant.
3. Add tests for variant selection, cancellation, branch admission, and response-body handling.
4. Implement the first Java variant and verify it with the pinned test/build path.
5. Run a pilot at 100/s and 800/s for `/io`, `/downstream`, and the three primary fan-out widths.
6. Freeze the matrix and run the three-repetition primary comparison.
7. Capture profiles only at selected stable and degraded points.
8. Repeat the selected configuration at two and four CPUs.
9. Analyze artifacts and update a new report; leave the Experiment 001 report unchanged.
10. Review the final conclusions against the acceptance criteria before declaring the experiment complete.

## Explicit non-goals

This experiment does not require PostgreSQL, a production network, a different web framework, native-image compilation, a language-version survey, or a general benchmark of Java and Go. Those changes would introduce separate variables and should be proposed as separate experiments.

## Baseline references

- Experiment 001 report: `/tmp/sut-final-comparison.kH31xH/comparison-report.md`
- Experiment 001 running learnings: [`001-concurrency-learnings.md`](001-concurrency-learnings.md)
- Experiment 001 incremental report: [`001-incremental-report-2026-10-11.md`](001-incremental-report-2026-10-11.md)
- Experiment 001 source: `experiments/001-virtual-threads-vs-goroutines`
