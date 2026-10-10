# Workloads

The first workload is a synthetic interruptible wait:

```text
GET /io?delay=50
```

It keeps the request active for approximately 50 milliseconds unless the request context or virtual-thread interrupt is cancelled. This is deliberately a control workload: it exercises high numbers of concurrently waiting requests without introducing a real downstream service, database, network, or disk dependency.

The pinned load generator is built from Vegeta `v12.13.0` in `vegeta/Dockerfile`. It is attached to the Compose network but has no published host port. The benchmark scripts use it to send the same request semantics to the Java and Go service containers.

The second workload is a deterministic downstream HTTP call:

```text
GET /downstream?delay=50
```

The Java or Go service synchronously calls the private `downstream` Compose service, which serves `GET /io?delay=50` after an interruptible 50 ms wait. Both SUT clients use HTTP/1.1 and a 500 ms downstream timeout. The dependency is identical for both runs and is not included in SUT CPU or memory summaries.

The downstream can also be configured for controlled scenarios through Compose: `success`, `error`, `slow`, and bounded `capacity`. The public SUT endpoint remains unchanged; scenario configuration is recorded in `run.json` and is never inferred from response data.

Future workloads should preserve equivalent request semantics and document their own downstream resource limits, timeout behavior, failure injection, and cleanup protocol before they are used for comparison.
