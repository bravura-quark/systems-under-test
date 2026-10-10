# Experiment 001 I/O Load Harness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a pinned open-loop `/io?delay=50` load harness that runs identical warm-up/rate tests against Java and Go, captures raw Vegeta results and independent Docker metrics, and summarizes measured data without conclusions.

**Architecture:** Build Vegeta CLI v12.13.0 from its official source tag in a pinned Go 1.26 container. Run it as a separate Compose service on the same private network as the selected system under test. A shell runner orchestrates one service/rate at a time, while a separate Docker stats sampler records container resource data.

**Tech Stack:** Docker Compose, Go 1.26, Vegeta CLI v12.13.0, POSIX shell, Python 3 standard library, JSONL/JSON artifacts.

---

### Task 1: Add the pinned load-generator image

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/workloads/vegeta/Dockerfile`
- Modify: `experiments/001-virtual-threads-vs-goroutines/docker-compose.yml`

**Steps:**

1. Build Vegeta from `github.com/tsenart/vegeta` tag `v12.13.0` using `golang:1.26.0-bookworm`.
2. Copy only the resulting static binary into a small runtime image with `vegeta` as the entrypoint.
3. Add a `loadgen` Compose service with explicit CPU/memory limits and no published ports.
4. Run `docker compose config` and `docker compose build loadgen`.
5. Verify `docker compose run --rm --no-deps loadgen -version` reports v12.13.0.
6. Commit with `git add workloads/vegeta/Dockerfile docker-compose.yml && git commit -m "build: add pinned vegeta load generator"`.

### Task 2: Add independent Docker stats collection

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/collect_container_stats.sh`

**Steps:**

1. Accept a container ID and JSONL output path.
2. Sample `docker stats --no-stream --format '{{json .}}'` once per second until terminated.
3. Wrap each Docker stats object with a UTC timestamp and flush each line immediately.
4. Handle container removal or empty stats output without corrupting the output file.
5. Run `bash -n scripts/collect_container_stats.sh`.
6. Commit with `git add scripts/collect_container_stats.sh && git commit -m "test: collect independent container metrics"`.

### Task 3: Add one-rate benchmark orchestration

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/benchmark_io.sh`
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/write_run_metadata.py`

**Steps:**

1. Accept `java` or `go`; configure internal Compose target names and host-independent service URLs.
2. Accept explicit environment overrides for rate, warm-up duration, measurement duration, timeout, connections, and output root.
3. Start only the selected service and wait for its Docker health status.
4. Send warm-up requests through the loadgen container and discard their results.
5. Start the independent stats sampler after warm-up and before measurement.
6. Run Vegeta at the configured open-loop rate against `/io?delay=50`, writing the raw attack stream to `attack.bin`.
7. Stop the sampler, generate `report.json` from the raw stream, write `run.json`, and clean up the service with a trap.
8. Record Git commit, tool versions, Docker image ID, service, target URL, rate, durations, timeout, connection limit, and timestamps.
9. Run a short validation against each service with output under `/tmp` and inspect all four artifacts.
10. Commit with `git add scripts/benchmark_io.sh scripts/write_run_metadata.py && git commit -m "feat: add one-rate io benchmark runner"`.

### Task 4: Add matrix execution and report summarization

**Files:**
- Create: `experiments/001-virtual-threads-vs-goroutines/scripts/compare_io.sh`
- Create: `experiments/001-virtual-threads-vs-goroutines/analysis/summarize_run.py`

**Steps:**

1. Make `compare_io.sh` run the same rate matrix against Java and Go sequentially, never sharing a running service container between measurements.
2. Keep the default matrix short for validation and make rates, durations, repetitions, and output root explicit environment inputs.
3. Make `summarize_run.py` read `report.json` and `container-stats.jsonl` and write a derived summary containing request count, throughput, success ratio, error count, p50/p95/p99 latency, and observed CPU/memory summaries.
4. Preserve raw values and units; do not emit labels such as faster, better, winner, or superior.
5. Test the summarizer against a checked-in small fixture or generated validation report without treating it as a benchmark result.
6. Commit with `git add scripts/compare_io.sh analysis/summarize_run.py && git commit -m "feat: add io benchmark matrix and summaries"`.

### Task 5: Document protocol and perform final verification

**Files:**
- Modify: `experiments/001-virtual-threads-vs-goroutines/README.md`
- Create: `experiments/001-virtual-threads-vs-goroutines/workloads/README.md`

**Steps:**

1. Document exact commands for building the loadgen, running one service/rate, running the matrix, and choosing an external output directory.
2. Document that `/io` is a synthetic blocking-wait control and not real downstream I/O.
3. Document warm-up exclusion, open-loop rates, independent stats collection, artifact schema, and limitations such as load-generator CPU/file-descriptor ceilings.
4. Run the full verification set:
   - `go test ./...`
   - `go vet ./...`
   - Java tests in the Gradle 9.8.1/JDK 25 container
   - `docker compose config`
   - `docker compose build java go loadgen`
   - Vegeta version check
   - short validation run for Java
   - short validation run for Go
   - `git diff --check`
5. Confirm no benchmark artifacts are accidentally added to Git unless explicitly intended.
6. Commit with `git add README.md workloads/README.md && git commit -m "docs: document io load harness"`.

