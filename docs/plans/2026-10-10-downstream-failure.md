# Experiment 001 Downstream Failure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add deterministic downstream failure, slow-response, and bounded-capacity scenarios and run them through the existing Java/Go comparison harness.

**Architecture:** Configure the private downstream Go service with `DOWNSTREAM_MODE` and `DOWNSTREAM_MAX_IN_FLIGHT`. Keep `/downstream?delay=50` as the public SUT workload, map dependency failures to `502`, preserve caller cancellation, and record scenario configuration in raw run metadata. Extend the matrix runner without changing the `/io` control.

**Tech Stack:** Go 1.26 `net/http`, Java 25 HTTP client, Docker Compose, Vegeta 12.13.0, shell, Python standard library, JUnit.

---

### Task 1: Implement and test downstream scenarios

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/internal/server/server.go`
- Modify: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/internal/server/server_test.go`
- Modify: `experiments/001-virtual-threads-vs-goroutines/infrastructure/downstream/cmd/server/main.go`

**Steps:**

1. Add failing tests for `error` mode returning `503`, `slow` mode exceeding the normal delay, and `capacity` mode rejecting a second concurrent request when the limit is one.
2. Run `go test ./...` in `infrastructure/downstream` and confirm the new tests fail because `Config` has no scenario fields.
3. Add `Mode` and `MaxInFlight` configuration. Implement `success`, `error`, `slow`, and semaphore-based `capacity` behavior while preserving cancellation and `/health`.
4. Parse `DOWNSTREAM_MODE` and `DOWNSTREAM_MAX_IN_FLIGHT` in the service main program.
5. Run `gofmt`, `go test ./...`, and `go vet ./...`.
6. Commit: `feat: add downstream failure scenarios`.

### Task 2: Record and execute scenarios in the harness

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/benchmark_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/compare_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/write_run_metadata.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/test_workload_metadata.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/docker-compose.yml`

**Steps:**

1. Extend the metadata test with expected scenario and capacity fields; run it and confirm it fails.
2. Add `DOWNSTREAM_MODE` and `DOWNSTREAM_MAX_IN_FLIGHT` to Compose and `run.json`.
3. Add `SCENARIOS`, defaulting to `success`, to the matrix runner. Skip non-success scenarios for `/io`; run all requested scenarios for `/downstream`.
4. Preserve the current default behavior and raw artifact layout.
5. Run the metadata test, shell syntax checks, Compose config, and a short one-scenario downstream run.
6. Commit: `feat: add downstream scenario matrix`.

### Task 3: Add scenario smoke coverage and documentation

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Modify: `experiments/001-virtual-threads-vs-goroutines/workloads/README.md`

**Steps:**

1. Add smoke assertions for the error and slow scenarios; run against the default success Compose service and confirm they fail until the wrapper can select a scenario.
2. Add a scenario argument to the smoke wrapper and pass the corresponding environment to Compose.
3. Document scenario commands, `502` mapping, capacity rejection, and the fact that dynamic recovery is not yet measured.
4. Run success, error, slow, and capacity smoke checks.
5. Commit: `docs: document downstream failure scenarios`.

### Task 4: Run verification and scenario matrix

**Files:**
- No planned production changes; inspect all generated artifacts.

**Steps:**

1. Run Go service tests/vet, downstream tests/vet, Java Gradle tests, Python metadata tests, shell syntax checks, and `git diff --check`.
2. Build Java, Go, downstream, and loadgen images; verify Vegeta version and Compose config.
3. Run short `/downstream?delay=50` validations for `success`, `error`, `slow`, and `capacity` against both services under `/tmp`.
4. Inspect `report.json`, `summary.json`, and `run.json` for every scenario; do not treat short runs as publication findings.
5. Confirm `results/` contains no generated artifacts and commit any final test/documentation adjustment.
