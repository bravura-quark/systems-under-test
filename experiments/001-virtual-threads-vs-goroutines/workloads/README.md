# Workloads

The first workload is a synthetic interruptible wait:

```text
GET /io?delay=50
```

It keeps the request active for approximately 50 milliseconds unless the request context or virtual-thread interrupt is cancelled. This is deliberately a control workload: it exercises high numbers of concurrently waiting requests without introducing a real downstream service, database, network, or disk dependency.

The pinned load generator is built from Vegeta `v12.13.0` in `vegeta/Dockerfile`. It is attached to the Compose network but has no published host port. The benchmark scripts use it to send the same request semantics to the Java and Go service containers.

Future workloads should preserve equivalent request semantics and document their own downstream resource limits, timeout behavior, failure injection, and cleanup protocol before they are used for comparison.
