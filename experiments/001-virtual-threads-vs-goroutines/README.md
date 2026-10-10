# Experiment 001: Java Virtual Threads vs Go Goroutines

This experiment compares equivalent HTTP services under increasing I/O concurrency. The runnable baseline includes `/health`, a synthetic cancellable wait, deterministic downstream and fan-out HTTP workloads, timeouts, containers, smoke tests, and a pinned open-loop load harness. The repository contains no checked-in benchmark results and makes no performance claims.

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
workloads/     Workload definitions and pinned load-generator configuration
infrastructure/ Future host/container setup and profiling support
scripts/       Smoke tests and benchmark orchestration
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

### `GET /downstream?delay=<milliseconds>`

The Java or Go service makes one synchronous HTTP/1.1 request to the private Compose service at `http://downstream:8080/io?delay=<milliseconds>`. A successful downstream response is mapped to the same success payload as `/io`. Downstream transport failures, timeouts, and non-2xx responses return `502 application/json`:

```json
{"status":"bad_gateway","delay_ms":50}
```

The downstream client uses a shared connection-capable client, a 500 ms request timeout, and bounded idle connection settings. In the Compose benchmark configuration, both clients use an idle connection limit of 100 per downstream host; Java receives this through `JAVA_TOOL_OPTIONS`. Incoming cancellation is propagated to the downstream request; a cancelled request does not produce a success response.

### `GET /fanout?delay=<milliseconds>&branches=<count>`

The Java or Go service starts one concurrent downstream HTTP request per branch and joins all branch results. `branches` must be between 1 and 16. All branches must return 2xx for the request to return the normal success payload; any downstream failure returns `502`. Request cancellation cancels the child requests and does not produce a success response. With `branches=4`, an incoming rate of `100/s` creates an effective downstream request rate of approximately `400/s`, so fan-out runs must report both rates.

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

## Run the I/O harness

The load generator is Vegeta `v12.13.0`, built from its tagged source with Go `1.26.0`. Build it with the services before running a measurement:

```sh
docker compose build java go loadgen
docker compose run --rm --no-deps loadgen -version
```

Run one short measurement. The output directory is intentionally outside the repository for validation runs:

```sh
RESULT_ROOT=/tmp/sut-validation \
WORKLOAD_PATH='/io?delay=50' \
WARMUP_DURATION=2s \
DURATION=5s \
RATE=100/s \
REQUEST_TIMEOUT=3s \
CONNECTIONS=100 \
scripts/benchmark_io.sh java
```

Use `go` instead of `java` for the Go service. Set `WORKLOAD_PATH='/downstream?delay=50'` or `WORKLOAD_PATH='/fanout?delay=50&branches=4'` to exercise real downstream HTTP. The runner starts one SUT, starts the downstream dependency for downstream and fan-out workloads, waits for health checks, sends a discarded warm-up, measures at an open-loop rate, samples SUT Docker stats independently, writes raw results, and removes the service containers when finished.

Run the same matrix sequentially against both services:

```sh
RESULT_ROOT=/tmp/sut-validation \
WORKLOAD_PATHS='/io?delay=50,/downstream?delay=50,/fanout?delay=50&branches=4' \
SCENARIOS=success \
RATES=10/s,100/s,500/s,1000/s,2000/s \
WARMUP_DURATION=2s \
DURATION=5s \
REPETITIONS=1 \
scripts/compare_io.sh
```

For a first smoke-sized matrix, override `WORKLOAD_PATHS`, `SCENARIOS`, `RATES`, `WARMUP_DURATION`, and `DURATION`. The default scenario is `success`; non-success scenarios apply to downstream-backed workloads, including `/fanout`. Use a fresh output directory for each experiment session.

Run the controlled dependency scenarios with:

```sh
RESULT_ROOT=/tmp/sut-failure-validation \
WORKLOAD_PATHS='/downstream?delay=50' \
SCENARIOS=success,error,slow,capacity \
DOWNSTREAM_MAX_IN_FLIGHT=1 \
RATES=10/s,100/s,500/s \
WARMUP_DURATION=2s DURATION=10s \
scripts/compare_io.sh
```

Scenario meanings:

- `success`: normal 50 ms downstream response.
- `error`: downstream returns `503`, which the SUT maps to `502`.
- `slow`: downstream exceeds the 500 ms client timeout, which the SUT maps to `502`.
- `capacity`: downstream admits only the configured number of in-flight requests and rejects overflow with `503`.

These modes are deterministic fault controls, not claims about a particular production dependency.

Each run is stored as `<result-root>/<run-id>/<service>/`:

| Artifact | Meaning |
| --- | --- |
| `attack.bin` | Raw Vegeta result stream; do not edit |
| `report.json` | Vegeta’s derived report |
| `container-stats.jsonl` | One Docker stats sample per line with a UTC timestamp |
| `run.json` | Git, image, tool, host, target, rate, timeout, and timing metadata |
| `summary.json` | Optional derived latency, success/error, throughput, CPU, and memory fields; generated by `compare_io.sh` or the command below |

Generate a summary for an individual run with:

```sh
python3 analysis/summarize_run.py \
  --run-dir /tmp/sut-validation/<run-id>/java \
  --output /tmp/sut-validation/<run-id>/java/summary.json
```

The summary treats a response with a 2xx status as successful. Vegeta’s top-level `success` value is retained as `all_requests_succeeded`; it is a boolean, not a request count.

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

The harness uses an open-loop request rate and runs Java and Go sequentially on the same Compose project. Vegeta runs in a separate container with its own explicit CPU and memory limits. Docker stats are sampled by a host-side process rather than by the load generator. For downstream runs, the same deterministic dependency image is used for both SUTs and its resource usage is kept separate from SUT metrics. These controls reduce, but do not eliminate, confounding from the host, Docker Desktop virtualization, network setup, scheduling, and measurement resolution.

The `/io` endpoint is a synthetic interruptible wait. It is a control workload for comparing request concurrency and cancellation behavior; it must not be described as equivalent to a real downstream HTTP service or database workload. `/downstream` adds one real socket and HTTP client path, while `/fanout` adds a controlled number of concurrent client paths; the dependency remains deterministic and local to the Compose network, so neither is a production service benchmark.

## Not measured yet

The harness can now collect throughput, p50/p95/p99 latency, CPU, memory, and status outcomes for the control, downstream HTTP, and fan-out HTTP workloads. This increment does not establish saturation points, comparative conclusions, profiling results, PostgreSQL, or production performance. Those belong to later, separately reproducible workload increments.
