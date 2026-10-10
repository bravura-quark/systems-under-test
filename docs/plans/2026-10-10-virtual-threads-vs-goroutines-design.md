# Systems Under Test 001: Virtual Threads vs Goroutines Design

## Goal

Build a reproducible baseline for comparing Java virtual-thread-per-request HTTP handling with idiomatic Go `net/http` request handling under high I/O concurrency. The first increment establishes equivalent service behavior and verification scaffolding; it does not produce benchmark results or performance conclusions.

## Scope of the first increment

- Java 25 service using Gradle and virtual-thread-per-request handling.
- Recent stable Go service using `net/http` and request-scoped goroutines.
- Equivalent `/health` and `/io?delay=50` HTTP endpoints.
- Bounded delay validation, request timeouts, and cancellation-aware simulated I/O.
- Dockerfiles and Docker Compose as the canonical execution path.
- Language-level tests and live-container smoke tests.
- README instructions and explicit documentation of fairness risks.

Downstream HTTP, fan-out, PostgreSQL, benchmark orchestration, profiling automation, and analysis are later increments.

## Architecture

The repository is a monorepo organized under `experiments/001-virtual-threads-vs-goroutines/` with independent `java` and `go` services. Both listen on port 8080 inside their containers and implement the same externally observable contract. Compose provides separate service targets, health checks, and explicit resource configuration. A shared smoke-test script exercises either live service without making language-specific assumptions.

The Java service uses the JDK HTTP server and an explicitly configured virtual-thread executor. Gradle Wrapper, Java toolchain configuration, and JUnit 5 provide reproducible builds and tests. The Go service uses the standard library, with a small `cmd/server` wiring entry point and testable internal server/handler code.

## HTTP contract

- `GET /health` returns HTTP 200 with a small stable JSON response and performs no simulated work.
- `GET /io?delay=<milliseconds>` accepts a non-negative, bounded delay. The baseline case is `delay=50`.
- Valid `/io` requests wait for the requested duration using an interruptible/cancellation-aware mechanism, then return HTTP 200 with the same small JSON shape in both implementations.
- Missing, malformed, negative, or excessive delay values return a client error with a stable JSON error shape.
- The service applies explicit read, write, idle, and request-handler timeout policies where supported by the runtime/server API.
- A disconnected client must be observable as cancellation/interruption and must not cause the handler to continue unbounded work.

The smoke tests verify status codes, content type, response shape, delay validation, health checks, and timeout/cancellation behavior. They do not compare latency or throughput.

## Cancellation and timeout model

Go receives cancellation through `http.Request.Context()`. The handler waits with a timer and selects between timer completion and `ctx.Done()`, stopping work on cancellation. Java uses interruption of the virtual-thread task, restores the interrupted status, and avoids returning a successful response after interruption. Server-level timeouts are defense in depth and are tested separately from client disconnect behavior.

## Containers and reproducibility

Docker Compose is the canonical execution path. Official toolchain/runtime images are pinned to explicit versions and the selected versions are recorded in the README. Runtime containers run as a non-root user where practical. Service ports, CPU/memory limits, and environment-configured timeout values are explicit rather than inherited from host defaults. The load generator is kept conceptually separate from the systems under test so its CPU and connection behavior can be controlled later.

## Testing strategy

Each service has unit tests for parsing, validation, response shape, delay completion, and cancellation/timeout paths. Build tests run inside the language-specific container/toolchain. A cross-language smoke script starts one service at a time and verifies the shared contract over HTTP. The README lists every command and states what each test proves.

## Fairness and confounding risks

Later benchmark runs must control or disclose:

- HTTP stack and JSON encoding differences.
- Connection reuse, client concurrency, load-generator CPU contention, and kernel/file-descriptor limits.
- Java JIT warm-up, garbage collector, Go runtime/toolchain version, and host thermal/frequency behavior.
- Container CPU/memory limits and any host scheduling interference.
- The fact that fixed-delay simulation isolates scheduler/request bookkeeping rather than real downstream network I/O.
- Matching request concurrency, connection limits, timeout policy, and workload mix rather than comparing raw thread/goroutine counts.
- Warm-up exclusion, repeated runs, sample counts, coordinated omission, and tail-latency confidence.

No result is considered a benchmark finding until raw results, environment metadata, run configuration, and analysis scripts are checked in.

## Incremental roadmap

1. Runnable baseline: services, contract tests, containers, smoke tests, and README.
2. Repeatable load generation: warm-up, repeated runs, raw result schema, and environment capture.
3. Metrics: throughput, p50/p95/p99 latency, CPU, memory, and error rates near saturation.
4. Profiling: Go `pprof`/`runtime/trace` and an appropriate Java profiler/flight recording workflow.
5. Workload expansion: downstream HTTP, fan-out, and PostgreSQL with matched resource limits.
6. Cancellation, backpressure, and failure-recovery experiments with publishable analysis.

