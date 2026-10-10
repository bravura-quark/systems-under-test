# Experiment 001 Downstream HTTP Workload Design

## Goal

Extend Experiment 001 from a synthetic interruptible wait to a controlled real-socket HTTP workload. Preserve `/io?delay=50` as the control and add equivalent `/downstream?delay=50` behavior to the Java and Go services.

## Scope

- Add a Compose-only deterministic downstream HTTP service.
- Add Java and Go downstream client handlers with equivalent semantics.
- Configure both clients for HTTP/1.1, bounded timeouts, bounded connection pools, and the same response contract.
- Propagate request cancellation to the downstream call.
- Extend the existing harness to select the workload path while preserving raw artifacts and metadata.
- Add unit, integration, and short validation coverage.

Failure injection, fan-out, PostgreSQL, and profiling are out of scope for this increment.

## Architecture

The downstream service listens on the private Compose network at `http://downstream:8080/io?delay=50`. It waits interruptibly for the requested delay and returns the same JSON payload used by the control endpoint:

```json
{"status":"ok","delay_ms":50}
```

The Java and Go SUT containers do not publish or discover a host port for this call. Each SUT exposes `/downstream?delay=50` to the load generator and makes one synchronous HTTP/1.1 request to the shared downstream service.

The downstream service is infrastructure, not a subject of comparison. It uses fixed CPU and memory limits, has a health check, and is started alongside exactly one SUT during a run.

## Client behavior

Java will create one shared JDK `HttpClient` and issue a synchronous request from the virtual-thread request handler. Go will create one shared `http.Client` and issue a request using the incoming `context.Context`. Both clients will use a bounded request timeout, HTTP/1.1, a bounded idle connection pool, and a small fixed response body.

For a successful downstream response, both SUTs return the existing success payload. Invalid parameters return `400`. Downstream timeout, transport failure, or non-success downstream status returns a consistent `502` response. If the caller disconnects or the request deadline expires, the SUT stops waiting and does not write a success response.

## Harness and artifacts

The benchmark runner will accept a workload path, defaulting to `/io?delay=50`. The comparison script will run the same rate matrix against both `/io?delay=50` and `/downstream?delay=50`, one SUT at a time. `run.json` will record the workload path and downstream client configuration so raw results remain interpretable.

The downstream container will be included in the run setup but its metrics will not be combined with SUT metrics. This prevents infrastructure work from being mistaken for application resource usage.

## Testing strategy

- Java unit tests: downstream success, malformed response, transport failure, timeout, and interrupted request.
- Go tests: downstream success, non-success response, transport failure, timeout, and request-context cancellation.
- Integration smoke tests: both SUTs reach the Compose downstream service and preserve the response contract.
- Harness validation: short Java and Go runs against both workload paths, with raw reports and summaries inspected.

The workload is still not a production database or service benchmark. The first publication-grade run will require repeated measurements, explicit saturation criteria, and profiling in a later increment.
