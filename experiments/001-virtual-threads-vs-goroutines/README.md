# Experiment 001: Java Virtual Threads vs Go Goroutines

This experiment compares equivalent HTTP services under increasing I/O concurrency. The current increment is only the runnable baseline: `/health`, cancellable simulated I/O, timeouts, containers, and smoke tests. It contains no benchmark results and makes no performance claims.

## Pinned baseline

- Java: 25
- Gradle: 9.8.1 via the Gradle Wrapper
- Go: 1.26.0
- HTTP implementation: JDK `HttpServer` with virtual threads vs Go standard-library `net/http`

The services intentionally use minimal dependencies. This makes the first comparison easier to inspect and keeps framework behavior out of the baseline.

## Layout

```text
java/          Java 25 service and Gradle build
go/            Go 1.26 service
workloads/     Future workload definitions and load-generator configuration
infrastructure/ Future host/container setup and profiling support
scripts/       Smoke tests and future benchmark orchestration
results/       Raw measured results only; empty until experiments run
analysis/      Reproducible analysis scripts and reports
```

## API contract

Both services listen on port `8080` in their containers.

### `GET /health`

Returns `200 application/json` with:

```json
{"status":"ok"}
```

### `GET /io?delay=<milliseconds>`

`delay` must be an integer from `0` through `5000`. The baseline workload is `delay=50`.

Success returns `200 application/json` with:

```json
{"status":"ok","delay_ms":50}
```

Missing, malformed, negative, or excessive values return `400 application/json` with an error object. The wait is interruptible: Go observes `request.Context()` cancellation, and Java observes virtual-thread interruption. The server also applies a 2-second request deadline, so `delay=5000` is a valid input but is expected to time out.

## Run with Docker Compose

From this directory:

```sh
docker compose build
docker compose up -d java
scripts/smoke.sh java
docker compose down
```

Run the Go service instead:

```sh
docker compose up -d go
scripts/smoke.sh go
docker compose down
```

The Java service is published on `localhost:8081`; Go is published on `localhost:8082`. Only one service needs to be running for the smoke test.

## Run language-level tests

Go can be tested directly when Go 1.26 is installed:

```sh
cd go
go test ./...
go vet ./...
```

Java tests require Java 25. The canonical command is run by the Java container during image build/test. With Java 25 available locally:

```sh
cd java
./gradlew test
```

## Go design notes

Each Go HTTP request already runs in its own runtime-managed goroutine. The `/io` handler does not create another goroutine: it owns a `time.Timer` and selects between the timer completing and `request.Context().Done()`.

`context.Context` is the request’s cancellation signal. It is cancelled when the client disconnects, the server deadline expires, or an upstream handler cancels it. The handler must stop waiting when that channel fires. A timer is stopped on the cancellation path so it does not remain active unnecessarily; there is no goroutine to leak.

`http.Server` timeouts are separate defense in depth. They bound header reads, body reads, response writes, and idle keep-alive time. Graceful shutdown lets active requests observe cancellation before the process exits.

## Fairness notes

This fixed-delay baseline isolates request scheduling and bookkeeping; it is not a real downstream network benchmark. Later runs must control warm-up, connection reuse, load-generator CPU, container CPU/memory, file-descriptor limits, JVM JIT/GC behavior, Go toolchain/runtime version, sample counts, and coordinated omission. Raw results and environment metadata must accompany any published finding.

## Not measured yet

This increment does not measure throughput, p50/p95/p99 latency, CPU, memory, error rates near saturation, profiling, downstream HTTP, fan-out, PostgreSQL, or comparative performance. Those belong to later, separately reproducible workload increments.

