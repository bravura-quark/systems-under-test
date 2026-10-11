# Experiment 001 Concurrency Learnings

This document is the running record of the concurrency concepts, measured findings, and interpretation limits from Experiment 001, which compares a Java 25 virtual-thread service with a Go 1.26 goroutine service. It complements the reproducible benchmark report and raw artifacts; it does not replace them.

## Current conclusion

The observed Java degradation is specific to this fan-out implementation and runtime stack under high concurrent downstream HTTP load. It is not evidence that Java virtual threads are universally slower than Go goroutines.

The Java service initially exhausted the heap under the 800 requests/s, four-branch workload. Explicit admission control reduced the failure severity, but Java still used substantially more CPU and memory and had higher tail latency than Go in the controlled runs.

## Workload model

The primary stress point is:

```text
/fanout?delay=50&branches=4
offered rate: 800 requests/s
downstream delay: approximately 50 ms
```

Each incoming fan-out request creates four downstream HTTP calls. The effective downstream offer is therefore approximately 3,200 branch calls/s.

The useful distinction is:

- A fan-out request is the client-visible operation.
- A branch is one downstream call created by that operation.
- A request permit limits active fan-out operations.
- A branch permit limits active downstream calls.

For four branches per request:

```text
active fan-out requests = branch permits / 4
```

## Concurrency versus throughput

Goroutines and virtual threads make waiting on I/O cheaper than dedicating one heavyweight operating-system thread per request. They do not remove downstream latency or make the downstream service complete work faster.

At a fixed offered rate, longer latency increases the number of operations in flight:

```text
in-flight work approximately equals rate multiplied by latency
```

For 800 requests/s:

- 50 ms latency implies approximately 40 fan-out requests in flight.
- 100 ms latency implies approximately 80 fan-out requests in flight.
- 500 ms latency implies approximately 400 fan-out requests in flight.

With a fixed admission cap, the number of active operations is bounded instead. Longer downstream delay then makes each permit remain occupied longer, reducing completed throughput and increasing capacity rejections.

## Why a branch cap of 128 rejects work

With 128 branch permits and four branches per request:

```text
128 / 4 = 32 active fan-out requests
32 / 0.05 seconds = approximately 640 requests/s
```

The offered rate is 800 requests/s, so the service cannot successfully complete every request within the 50 ms workload model. The resulting 503 responses are expected backpressure, not random Go failures.

With 256 branch permits:

```text
256 / 4 = 64 active fan-out requests
64 / 0.05 seconds = approximately 1,280 requests/s
```

That provides enough modeled capacity for the 800 requests/s offer, subject to runtime, network, and downstream overhead.

## Measured findings

The first cap sweep used a request cap of 256 and branch caps implicitly equal to the request cap. It ran two repetitions at 800 requests/s for each cap.

| Request cap | Java success | Java p99 | Java CPU | Go success | Go p99 |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 64 | 97.09% | 104 ms | 83% | 100% | 54 ms |
| 128 | 95.55% | 220 ms | 93% | 100% | 55 ms |
| 256 | 92.30% | 557 ms | 100% | 100% | 54 ms |
| 512 | 73.33% | 1,253 ms | 97% | 100% | 55 ms |

Raw artifacts: `/tmp/sut-cap-sweep-20261011`

The branch-cap sweep fixed the request cap at 64 and varied total active branch permits.

| Branch cap | Java success | Java p99 | Java CPU | Go success | Go p99 |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 128 | 73.52% | 70 ms | 69% | 75.09% | 54 ms |
| 256 | 97.21% | 102 ms | 83% | 100% | 54 ms |
| 512 | 95.77% | 124 ms | 84% | 100% | 54 ms |

Raw artifacts: `/tmp/sut-branch-cap-sweep-20261011`

The 128 branch cap limited both services to roughly 32 active four-branch requests, so both rejected a substantial fraction of the 800 requests/s offer. A branch cap of 256 was the best observed Java setting in this sweep, but it is an experimental result rather than a universal tuning recommendation.

## Root-cause evidence

The focused Java run without effective admission control exited with:

```text
java.lang.OutOfMemoryError: Java heap space
```

The earlier Java JFR was useful as contextual evidence but too sparse for precise method attribution. It showed no virtual-thread-pinned events in the captured recording. The Go CPU profile was dominated by ordinary HTTP socket, response-writing, and scheduler work rather than an obvious pathological hotspot.

The strongest evidence is the controlled behavior across caps: Java CPU, memory, tail latency, and failures worsen as more fan-out work is admitted. This supports allocation, HTTP-client coordination, and runtime resource pressure as the working explanation. It does not isolate one JDK method as the sole cause.

## Interpretation limits

- The services use different standard-library implementations: JDK `HttpServer` and `HttpClient` versus Go `net/http`.
- The fan-out branch tasks use blocking Java `HttpClient.send()` calls on virtual threads; virtual threads reduce thread cost but do not eliminate HTTP, allocation, connection, or garbage-collection cost.
- The benchmark uses one CPU and a 512 MB container for each SUT.
- A 503 capacity response is a deliberate overload-control result, not a downstream error.
- The measurements are open-loop offered-load results. They are not maximum-throughput proofs or general language rankings.
- The downstream service is deterministic and local to the Compose network; results should not be generalized to production databases or networks.

## Current design decisions

- Keep request and branch admission limits explicit and symmetric between Java and Go.
- Reserve all branches before starting a fan-out request; do not partially start a request that cannot reserve its full branch set.
- Record both caps in `run.json` alongside workload, image, tool, and timing metadata.
- Keep raw measurement directories outside Git and preserve the original comparison report unchanged.

## Next investigation

## Runtime attribution increment

The selected controlled point used request cap 64, branch cap 256, 800 requests/s, four branches, and a 50 ms downstream delay. The raw captures are in `/tmp/sut-final-runtime-attribution-20261011`.

The Java Flight Recorder capture showed substantial thread parking, allocation sampling, and garbage-collection activity, plus Java HTTP-client/future coordination in the sampled methods. It showed zero `jdk.VirtualThreadPinned` events in this recording. The Go capture was dominated by ordinary `net/http` socket reads/writes, scheduling, and garbage collection; its heap profile showed the expected HTTP transport, buffers, and goroutine-stack allocations. These profiles support a broad resource-pressure explanation, but do not prove that one specific Java method is the sole bottleneck.

## Delay/branch sensitivity increment

Raw artifacts: `/tmp/sut-final-sensitivity-20261011`. This was one repetition per point at 800 requests/s, with request cap 64 and branch cap 256. The matrix varied downstream delay across 10, 50, 100, and 250 ms and fan-out width across 1, 4, and 8 branches.

| Delay | Branches | Java success / p99 | Go success / p99 | Interpretation |
| ---: | ---: | ---: | ---: | --- |
| 10 ms | 1 | 100% / 19 ms | 100% / 13 ms | Both stable; Java costs more CPU/memory. |
| 10 ms | 4 | 88% / 160 ms | 100% / 14 ms | Java reaches CPU saturation earlier. |
| 10 ms | 8 | 47% / 165 ms | 100% / 26 ms | Wider Java fan-out amplifies coordination/resource pressure. |
| 50 ms | 1 | 99.9% / 63 ms | 100% / 55 ms | Single downstream call remains comparable. |
| 50 ms | 4 | 76% / 160 ms | 100% / 54 ms | Controlled cap prevents heap collapse, but Java still degrades. |
| 50 ms | 8 | 38% / 176 ms | 75% / 56 ms | Both hit branch capacity; Java has less headroom. |
| 100 ms | 1 | 77% / 107 ms | 78% / 103 ms | The 64-request cap is now the primary limit for both. |
| 250 ms | 1 | 32% / 254 ms | 32% / 253 ms | Results match the modeled cap/latency ceiling. |

The omitted 4- and 8-branch rows follow the same capacity pattern: at 250 ms, success is approximately 32% for four branches and 16% for eight branches for both services. At 100 ms, eight branches reduce both services to roughly 39% success. These are expected admission-control outcomes, not implementation failures.

The sensitivity increment therefore separates two effects: increasing delay consumes permits for longer in both implementations, while increasing fan-out width exposes an earlier Java resource/coordination limit at low and medium delays. The experiment still cannot distinguish the relative contributions of the JDK HTTP client, connection-pool behavior, allocation/GC, or the one-core container without a separately designed component-level experiment.

## Remaining scope

Experiment 001 now has a reproducible baseline, downstream/fan-out workload matrix, controlled failure and capacity scenarios, symmetric request/branch admission control, cap sweeps, runtime profiles, and delay/branch sensitivity evidence. A deeper JDK HTTP-client component comparison or multi-core scaling study would be a new experiment rather than a required completion of this one.
