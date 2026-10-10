# Virtual Threads vs Goroutines Baseline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the smallest reproducible Java 25 and Go 1.26 HTTP services for Experiment 001, with equivalent `/health` and cancellable `/io` behavior, Docker Compose execution, smoke tests, and no benchmark claims.

**Architecture:** Two independent standard-library services live under `experiments/001-virtual-threads-vs-goroutines/{java,go}`. Java uses a Gradle Wrapper build and the JDK HTTP server with a virtual-thread-per-request executor; Go uses `net/http` with request-context cancellation. Compose builds and runs either service with explicit health checks and resource limits, while a shell smoke test validates the shared contract.

**Tech Stack:** Java 25, Gradle 9.8.1, JUnit 5, Go 1.26.0, Docker Compose, POSIX shell, Python 3 for smoke-test HTTP/cancellation helpers if available.

---

### Task 1: Create the experiment skeleton and repository metadata

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Create: `experiments/001-virtual-threads-vs-goroutines/.gitignore`
- Create: `experiments/001-virtual-threads-vs-goroutines/results/.gitkeep`
- Create: `experiments/001-virtual-threads-vs-goroutines/analysis/.gitkeep`
- Create: `experiments/001-virtual-threads-vs-goroutines/workloads/.gitkeep`
- Create: `experiments/001-virtual-threads-vs-goroutines/infrastructure/.gitkeep`

**Steps:**

1. Add the requested directory layout and keep future results/analysis/workload directories empty except for `.gitkeep` files.
2. Document the current baseline scope, pinned toolchain versions, canonical Docker commands, endpoint contract, and explicit statement that no benchmark results exist yet.
3. Add ignores for build outputs, Gradle caches, Go binaries, profiles, logs, and generated result files while leaving raw checked-in results possible.
4. Run `git status --short` and verify only the intended skeleton files are untracked.
5. Commit with `git add experiments/001-virtual-threads-vs-goroutines && git commit -m "chore: scaffold experiment 001"`.

### Task 2: Add the Java Gradle project and contract tests

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/java/settings.gradle.kts`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/build.gradle.kts`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/gradle.properties`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Server.java`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Api.java`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/test/java/sut/experiment/ApiTest.java`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/gradlew`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/gradlew.bat`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/gradle/wrapper/gradle-wrapper.properties`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/gradle/wrapper/gradle-wrapper.jar`

**Steps:**

1. Use a Dockerized Gradle bootstrap because Gradle is not installed on the host; generate a Gradle 9.8.1 wrapper and verify `./gradlew --version` reports 9.8.1.
2. Configure the Java toolchain for Java 25, the Application plugin, JUnit 5, UTF-8 compilation, and a stable application main class.
3. Write tests first for valid delay parsing, default delay behavior, invalid/missing/negative/excessive delay rejection, stable success/error JSON, and interruption-aware wait behavior.
4. Run the focused Java tests and confirm they fail for the missing implementation.
5. Implement an `Api` component that owns the shared contract and returns response data/status without coupling parsing to socket operations.
6. Implement `Server` with `HttpServer`, an explicit virtual-thread executor, request/response timeouts, bounded request-body behavior, and graceful shutdown on JVM termination.
7. Implement `/health` as a no-work response and `/io` as an interruptible virtual-thread sleep that catches `InterruptedException`, restores the interrupt flag, and stops before writing a success response.
8. Run `./gradlew test` and confirm all Java tests pass.
9. Commit with `git add experiments/001-virtual-threads-vs-goroutines/java && git commit -m "feat: add Java virtual-thread baseline service"`.

### Task 3: Add the Go module and contract tests

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/go/go.mod`
- Create: `experiments/001-virtual-threads-vs-goroutines/go/cmd/server/main.go`
- Create: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server.go`
- Create: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server_test.go`

**Steps:**

1. Set the module’s Go version to 1.26 and keep dependencies in the standard library for the baseline.
2. Write tests first for valid/default delay parsing, invalid/missing/negative/excessive values, response shape, cancellation before timer completion, and handler timeout behavior.
3. Run `go test ./...` and confirm the focused tests fail for the missing implementation.
4. Implement a small `Server` type that exposes an `http.Handler`, owns the request timeout policy, and keeps `main` limited to configuration and lifecycle wiring.
5. Implement `/health` as a JSON response and `/io` with `time.NewTimer` plus a `select` over `timer.C` and `r.Context().Done()`.
6. Explain in comments why the timer is stopped/drained when cancellation wins, why the request context is the cancellation source, and why no extra goroutine is needed for the handler.
7. Configure `http.Server` read-header, read, write, and idle timeouts and graceful shutdown on SIGINT/SIGTERM.
8. Run `gofmt -w .`, `go vet ./...`, and `go test ./...`; confirm all Go checks pass.
9. Commit with `git add experiments/001-virtual-threads-vs-goroutines/go && git commit -m "feat: add Go goroutine baseline service"`.

### Task 4: Add reproducible container builds and Compose orchestration

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/java/Dockerfile`
- Create: `experiments/001-virtual-threads-vs-goroutines/go/Dockerfile`
- Create: `experiments/001-virtual-threads-vs-goroutines/docker-compose.yml`
- Create: `experiments/001-virtual-threads-vs-goroutines/.dockerignore`

**Steps:**

1. Build Java in a Gradle/JDK 25 builder or JDK 25 builder using the checked-in Gradle Wrapper, then run the application from an explicit Java 25 runtime image.
2. Build Go in a pinned Go 1.26.0 builder and run a statically linked binary in a minimal pinned runtime image where health-check tooling remains available.
3. Use explicit image tags/digests or documented exact tags, expose port 8080, configure non-root runtime users where practical, and avoid mounting source into runtime containers.
4. Define `java` and `go` Compose services with separate host ports, health checks, restart disabled for tests, timeout environment values, and explicit CPU/memory limits suitable for smoke tests.
5. Run `docker compose config` and verify the rendered configuration contains both services, health checks, resource limits, and no accidental host-source mounts.
6. Build both services with `docker compose build` and record only build output, not performance conclusions.
7. Commit with `git add experiments/001-virtual-threads-vs-goroutines/{java/Dockerfile,go/Dockerfile,docker-compose.yml,.dockerignore} && git commit -m "build: containerize experiment 001 services"`.

### Task 5: Add live-container smoke tests

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.sh`
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/cancel_request.py` or a standard-library shell/Python helper only if needed

**Steps:**

1. Make `smoke.sh` accept `java` or `go`, start only the selected Compose service, wait for its health check, and clean up with a trap.
2. Check `/health`, `/io?delay=0`, and `/io?delay=50` for status, content type, and expected JSON keys.
3. Check malformed, negative, and excessive delay values return the agreed client error.
4. Exercise client cancellation by opening a delayed request and closing the client before 50 ms; make the test assert only that the client disconnects promptly and the service remains healthy, not that internal work can be directly observed from HTTP.
5. Exercise the configured server timeout with a deliberately longer delay and verify a timeout response/connection close consistent with the documented contract.
6. Run `shellcheck` if available, otherwise run `bash -n scripts/smoke.sh`, and run the smoke test against both services.
7. Commit with `git add experiments/001-virtual-threads-vs-goroutines/scripts && git commit -m "test: add cross-language smoke checks"`.

### Task 6: Complete documentation and verification

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Modify: `docs/plans/2026-10-10-virtual-threads-vs-goroutines-design.md` only if implementation constraints require clarification

**Steps:**

1. Document exact commands for Java tests, Go tests, Compose build, service startup, smoke tests, and cleanup.
2. Add a Go design-notes section explaining `context.Context`, timer ownership, standard `net/http` request goroutines, server timeouts, and graceful shutdown.
3. Add a “What is not measured yet” section covering throughput, latency percentiles, CPU, memory, errors under saturation, profiling, and downstream workloads.
4. Run the full available verification set from the experiment directory:
   - `go test ./...`
   - `go vet ./...`
   - `./gradlew test` inside the Java container or with a Java 25 environment
   - `docker compose config`
   - `docker compose build`
   - `scripts/smoke.sh java`
   - `scripts/smoke.sh go`
5. Inspect `git diff --check`, `git status --short`, and the final file tree. Report any unavailable checks explicitly rather than inferring their result.
6. Commit documentation with `git add experiments/001-virtual-threads-vs-goroutines/README.md && git commit -m "docs: explain experiment 001 baseline"`.

