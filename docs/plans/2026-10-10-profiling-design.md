# Experiment 001 Profiling Design

## Decision

Add opt-in profiling paths that do not participate in normal benchmark runs:

- Go serves the standard library `net/http/pprof` index, CPU profile, heap profile, goroutine profile, and `runtime/trace` on a separate diagnostics listener when `PPROF_ADDR` is set.
- Java accepts an opt-in `JAVA_JFR_OPTIONS` fragment. Compose keeps the existing HTTP client pool settings and appends the supplied JFR options, writing recordings to a host directory mounted at `/profiles`.
- A profiling script starts one SUT and the deterministic downstream, waits for health, drives a selected workload, captures Go profiles over the measurement interval, and leaves JFR/profile artifacts outside Git.

## Fairness and safety

Profiling is never enabled by the ordinary comparison harness. The profiling run records the tool mode and workload separately because pprof and JFR add observer overhead. The Go diagnostics listener is bound independently from the request listener, and JFR output uses a caller-supplied host directory. The same CPU and memory limits remain in force.

## Evidence

The final report may use profiles to explain behavior, but profiles do not establish a performance conclusion by themselves. Latency, throughput, status, CPU, and memory still come from the open-loop benchmark artifacts.
