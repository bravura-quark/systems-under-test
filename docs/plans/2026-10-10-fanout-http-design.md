# Experiment 001 Fan-out HTTP Workload Design

## Goal

Add a controlled fan-out workload that makes each SUT request issue multiple concurrent downstream HTTP calls, allowing the comparison to include nested virtual-thread and goroutine scheduling.

## Contract

The SUT exposes `GET /fanout?delay=50&branches=4`. It validates `delay` and `branches` (`1..16`), starts exactly `branches` downstream calls to `http://downstream:8080/io?delay=50`, and returns the existing success payload only after all branches succeed. Any downstream failure maps to `502`; caller cancellation stops the child calls without writing a success response.

Java launches one virtual thread per branch from the request handler and joins the futures. Go launches one goroutine per branch, derives child contexts from the request context, and joins the results. Both use the already configured shared downstream HTTP client and HTTP/1.1 settings.

## Harness

Generalize workload selection to `/fanout?delay=50&branches=4` and record `fanout_branches` in `run.json`. The downstream service remains the same dependency and is not included in SUT summaries. Fan-out runs use the same open-loop request rate; the effective downstream request rate is documented as request rate multiplied by branch count.

## Scope

This increment covers success, cancellation, and downstream failure propagation for fan-out. It does not add heterogeneous branches, retries, or database work. Those remain later workloads.
