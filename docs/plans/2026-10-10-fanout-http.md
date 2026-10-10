# Experiment 001 Fan-out HTTP Workload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add equivalent concurrent downstream fan-out to Java and Go and expose it through the reproducible harness.

**Architecture:** Add `/fanout?delay=50&branches=4`. Java submits one synchronous downstream call per branch to a virtual-thread executor; Go starts one goroutine per branch with derived cancellation contexts. Both join all branches, map failures to `502`, and preserve the existing `/io` and `/downstream` paths.

**Tech Stack:** Java 25 virtual threads and `HttpClient`, Go 1.26 `net/http`, Docker Compose, Vegeta, JUnit, Go tests, Python metadata.

---

### Task 1: Add failing fan-out tests and Go implementation

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server_test.go`
- Modify: `experiments/001-virtual-threads-vs-goroutines/go/internal/server/server.go`

1. Add tests for four downstream calls, invalid branch counts, downstream failure mapping, and request cancellation. Run the targeted Go tests and confirm they fail because `/fanout` is absent.
2. Implement branch parsing, one child goroutine per branch, shared client calls, error collection, and cancellation propagation.
3. Run `gofmt`, targeted tests, `go test ./...`, and `go vet ./...`.
4. Commit `feat: add go fanout http workload`.

### Task 2: Add failing fan-out tests and Java implementation

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Server.java`
- Modify: `experiments/001-virtual-threads-vs-goroutines/java/src/main/java/sut/experiment/Api.java`
- Create: `experiments/001-virtual-threads-vs-goroutines/java/src/test/java/sut/experiment/FanoutTest.java`

1. Add real local HTTP server tests for four branch requests, invalid branches, downstream failure mapping, and interruption while branches are waiting. Confirm the Gradle test fails before the route exists.
2. Implement one virtual-thread child task per branch, join all futures, cancel remaining tasks on interruption/failure, and map failures to `502`.
3. Run the Gradle JDK 25 test suite.
4. Commit `feat: add java fanout http workload`.

### Task 3: Generalize metadata, smoke tests, and documentation

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/benchmark_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/compare_io.sh`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/write_run_metadata.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/test_workload_metadata.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/scripts/smoke.py`
- Modify: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Modify: `experiments/001-virtual-threads-vs-goroutines/workloads/README.md`

1. Add `FANOUT_BRANCHES` and metadata assertions; run the metadata test and confirm the new fields fail before implementation.
2. Add fan-out target selection and record branch count. Keep downstream service startup and scenario behavior unchanged.
3. Add success, invalid-branch, and failure smoke coverage.
4. Document effective downstream request multiplication and the fan-out contract.
5. Run smoke tests and short fan-out validation for Java and Go.
6. Commit `feat: add fanout workload to harness`.

### Task 4: Verify the increment

Run both Go modules’ tests/vet, the Java Gradle suite, metadata/syntax checks, Compose config/build, both service smoke tests, and a short fan-out matrix. Inspect raw artifacts and confirm no results enter Git.
