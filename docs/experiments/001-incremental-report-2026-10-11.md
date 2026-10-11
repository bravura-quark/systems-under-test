# Experiment 001 Incremental Report — 2026-10-11

This report records the completed increments after the original comparison report. It preserves the original results and adds symmetric fan-out admission control, cap sweeps, runtime attribution, and a bounded delay/branch sensitivity matrix.

## Reproducibility

- Java 25; Go 1.26.0; Gradle 9.8.1; Vegeta 12.13.0.
- Same Compose dependency, load generator, open-loop rate, request timeout, and container limits for both services.
- Sensitivity run: 800 requests/s, 3 s warm-up, 5 s measurement, one repetition, request cap 64, branch cap 256.
- Source revision recorded by the run metadata: `348bab7ca8fbb2c610a1ff0cba9de79d2c591fd9`.
- Raw artifacts: `/tmp/sut-final-sensitivity-20261011`.
- Runtime artifacts: `/tmp/sut-final-runtime-attribution-20261011`.
- Earlier cap sweeps: `/tmp/sut-cap-sweep-20261011` and `/tmp/sut-branch-cap-sweep-20261011`.

## Changes made

Both services now have two explicit, symmetric controls:

1. `FANOUT_MAX_IN_FLIGHT` limits active fan-out requests.
2. `FANOUT_MAX_BRANCHES_IN_FLIGHT` reserves all branches before a request starts downstream work.

An exhausted bound returns an explicit `503` capacity response. Tests cover both request-level and all-or-nothing branch admission. Benchmark metadata records both values.

## Sensitivity results

The table gives success ratio and p99 latency. The complete per-run data is in each raw run directory’s `summary.json`.

| Delay | Branches | Java | Go |
| ---: | ---: | ---: | ---: |
| 10 ms | 1 | 100% / 19 ms | 100% / 13 ms |
| 10 ms | 4 | 88% / 160 ms | 100% / 14 ms |
| 10 ms | 8 | 47% / 165 ms | 100% / 26 ms |
| 50 ms | 1 | 99.9% / 63 ms | 100% / 55 ms |
| 50 ms | 4 | 76% / 160 ms | 100% / 54 ms |
| 50 ms | 8 | 38% / 176 ms | 75% / 56 ms |
| 100 ms | 1 | 77% / 107 ms | 78% / 103 ms |
| 100 ms | 4 | 73% / 165 ms | 77% / 105 ms |
| 100 ms | 8 | 37% / 163 ms | 39% / 104 ms |
| 250 ms | 1 | 32% / 254 ms | 32% / 253 ms |
| 250 ms | 4 | 32% / 260 ms | 32% / 253 ms |
| 250 ms | 8 | 16% / 256 ms | 16% / 254 ms |

The 250 ms results are consistent with the modeled capacity ceiling: a 64-request cap and 256 branch permits cannot complete 800 requests/s when each request holds permits for roughly 250 ms. At lower delays, wider Java fan-out reaches high CPU and memory pressure earlier. Go remains stable through more of the same envelope, but also shows expected capacity rejection once delay and branch width consume the shared branch budget.

## Runtime evidence

The Java JFR capture contained substantial `ThreadPark`, allocation-sample, and garbage-collection activity, along with JDK HTTP-client/future coordination in sampled methods. It recorded zero virtual-thread-pinned events. The Go CPU and heap profiles showed ordinary `net/http` socket, scheduler, transport, buffer, and goroutine-stack work. This supports the conclusion that the Java gap is resource/coordination pressure in this HTTP fan-out stack, not virtual-thread pinning. It does not identify a single causal method.

## Conclusion and boundary

The remaining result is intentionally narrow: under the tested one-core, 512 MiB containers and local deterministic downstream, Java virtual threads performed well for simple waits and single downstream calls, while the Java JDK HTTP-client/fan-out path degraded sooner as concurrent branch work increased. Explicit admission control converted catastrophic heap exhaustion into observable backpressure, making the comparison safer and more reproducible.

This does not establish a universal Java-versus-Go performance ranking. A component-level JDK HTTP-client comparison, alternative connection-pool configuration, or multi-core scaling study is outside Experiment 001 and should be treated as a new experiment.
