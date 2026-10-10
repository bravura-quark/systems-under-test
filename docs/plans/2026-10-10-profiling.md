# Experiment 001 Profiling Implementation Plan

**Goal:** Add opt-in Go pprof/runtime-trace and Java JFR capture without changing ordinary comparison runs.

### Task 1: Go diagnostics

- Add failing tests for the diagnostics listener configuration and profile endpoints.
- Add `PPROF_ADDR` support, standard pprof handlers, and runtime/trace support.
- Run Go tests and vet; commit `feat: add opt-in go profiling`.

### Task 2: Java JFR configuration

- Add Compose-controlled JFR options and a host-mounted `/profiles` directory.
- Preserve the existing Java HTTP client settings and allow an opt-in recording fragment.
- Run Compose config/build; commit `feat: add opt-in java jfr capture`.

### Task 3: Reproducible profiling runner and docs

- Add a script that starts one service plus downstream, drives a selected workload, and captures Go and Java artifacts outside Git.
- Record profiling metadata and document observer overhead and artifact handling.
- Run a short profile capture for both services; commit `feat: add profiling capture runner`.

### Task 4: Verify increment

Run both Go module tests/vet, Java Gradle tests, Compose config/build, normal smoke tests, and short profile captures. Confirm no generated profiles enter Git.
