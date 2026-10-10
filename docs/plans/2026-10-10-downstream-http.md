# Experiment 001 Downstream HTTP Workload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a deterministic downstream HTTP workload so Java virtual-thread request handlers and Go goroutine request handlers perform equivalent real network I/O.

**Architecture:** Add a private Compose-only Go downstream service with a cancellable delayed `/io` endpoint. Add `/downstream` handlers to both SUTs that make one synchronous HTTP/1.1 request to that service using shared clients, bounded timeouts, and equivalent response/error semantics. Generalize the existing benchmark runner to select `/io` or `/downstream` while preserving raw artifacts and metadata.

**Tech Stack:** Java 25 `java.net.http.HttpClient`, Go 1.26 `net/http`, Docker Compose, Vegeta 12.13.0, Gradle, Go tests, JUnit, Python standard library.

---

### Task 1: Add the deterministic downstream service

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/go.mod`
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/cmd/server/main.go`
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/internal/server/server.go`
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/internal/server/server_test.go`
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/Dockerfile`
- Modify: `experiments/001-virtual-threads-vs-goroutines/docker-compose.yml`

**Step 1: Write the failing tests**

Add tests for `/health`, successful `/io?delay=50`, invalid delay values, and a cancelled request that does not write a success response. Run `go test ./...` from `infrastructure/downstream`; it must fail because the module and handler do not yet exist.

**Step 2: Implement the minimal service**

Implement a small `net/http` handler with the same delay validation and interruptible timer behavior as the control endpoint. Add graceful shutdown, a static Go image, a health check, and a Compose service with fixed CPU/memory limits and no published port.

**Step 3: Verify and commit**

Run `go test ./...`, `go vet ./...`, `docker compose config`, and `docker compose build downstream`. Commit with `feat: add deterministic downstream http service`.

### Task 2: Add the Go downstream client path

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server.go`
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server_test.go`
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/cmd/server/main.go`

**Step 1: Write the failing tests**

Using real `httptest.Server` instances and `http.Client` behavior, add tests for downstream success, non-2xx mapping to `502`, timeout mapping to `502`, incoming request cancellation, and unchanged `/io` behavior. Run `cd go && go test ./internal/server -run 'Test.*Downstream|TestIO' -count=1`; it must fail because `/downstream` does not exist.

**Step 2: Implement the minimal client path**

Extend the handler configuration with downstream URL and timeout. Construct one shared `http.Client` and transport in `NewHandler`, using HTTP/1.1, keep-alive, bounded idle connections, and the configured timeout. Add `/downstream` handling that uses `r.Context()`, maps transport/timeouts and non-success statuses to `502`, and preserves cancellation. Keep `/io` on its existing timer path.

**Step 3: Verify and commit**

Run `cd go && go test ./internal/server -run 'Test.*Downstream|TestIO' -count=1`, `go test ./...`, and `go vet ./...`. Commit with `feat: add go downstream http workload`.

### Task 3: Add the Java downstream client path

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/DownstreamClient.java`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/test/java/sut/experiment/DownstreamClientTest.java`
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Server.java`
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Api.java`
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/src/test/java/sut/experiment/ApiTest.java`

**Step 1: Write the failing tests**

Use a real local `HttpServer` fixture rather than a mocked client. Test downstream success, non-2xx response, timeout, interruption of a virtual-thread request while waiting, and unchanged `/io` behavior. Run the Gradle test container; it must fail because the client and route do not exist.

**Step 2: Implement the minimal client path**

Create one shared JDK `HttpClient` configured for HTTP/1.1 and bounded connect/request timeouts. Send synchronously from the virtual-thread handler. Map downstream failures to `502`, preserve `InterruptedException`, and avoid writing a success response after cancellation. Configure the downstream base URL through `DOWNSTREAM_URL`, defaulting to the Compose service URL, and add `/downstream` without changing `/io`.

**Step 3: Verify and commit**

Run `docker run --rm -v "$PWD":/workspace -w /workspace gradle:9.8.1-jdk25 gradle test`. Commit with `feat: add java downstream http workload`.

### Task 4: Wire Compose and generalize the benchmark harness

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/docker-compose.yml`
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/Dockerfile`
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/Dockerfile`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/benchmark_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/compare_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/write_run_metadata.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/analysis/summarize_run.py`

**Step 1: Write failing harness assertions**

Add checks that the default target remains `/io?delay=50`, `WORKLOAD_PATH=/downstream?delay=50` selects the downstream route, metadata records the selected workload and client configuration, and the runner starts and cleans up both the SUT and downstream service. Confirm the new assertions fail before implementation.

**Step 2: Implement workload selection**

Add `WORKLOAD_PATH` with the control as the default. Derive the internal target from the selected SUT and path. Start `downstream` alongside the selected SUT for downstream runs, wait for both health checks, and preserve one-SUT-at-a-time execution. Pass workload and client settings through metadata. Keep downstream resource samples separate from SUT summaries.

**Step 3: Verify and commit**

Run `docker compose config`, `docker compose build java go downstream loadgen`, and a short Java downstream run under `/tmp`. Confirm the four raw artifacts are produced and both service containers are cleaned up. Commit with `feat: benchmark downstream http workload`.

### Task 5: Add integration smoke coverage and documentation

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Modify: `experiments/001-virtual-threads-vs-goroutines/workloads/README.md`

**Step 1: Write failing integration assertions**

Extend smoke coverage for `/downstream?delay=0`, `/downstream?delay=50`, and downstream timeout/error mapping. Run before the route and Compose wiring are complete and confirm the new assertions fail.

**Step 2: Implement smoke invocation and documentation**

Update the shell wrapper to start downstream when testing Java or Go. Document synchronous client behavior, timeout/error mapping, workload selection, and the limitation that downstream is a controlled dependency rather than a production system.

**Step 3: Verify and commit**

Run short downstream smoke tests against both services, then commit with `docs: document downstream http workload`.

### Task 6: Full verification and handoff

**Files:**
- No planned production changes; inspect the complete diff and generated-artifact state.

Run:

```sh
cd experiments/001-virtual-threads-vs-goroutines/go
go test ./...
go vet ./...
cd ../infrastructure/downstream
go test ./...
go vet ./...
cd ../../java
docker run --rm -v "$PWD":/workspace -w /workspace gradle:9.8.1-jdk25 gradle test
cd ..
docker compose config
docker compose build java go downstream loadgen
docker compose run --rm --no-deps loadgen -version
git diff --check
git status --short
```

Run short control and downstream validations for both services under `/tmp`, inspect every raw artifact and summary, and confirm no generated validation files are under `results/`. Report exactly what passed and do not treat short validation numbers as benchmark findings.
