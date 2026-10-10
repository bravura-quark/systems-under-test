# Experiment 001 Downstream Failure and Capacity Design

## Goal

Extend the real downstream HTTP workload with deterministic dependency scenarios that exercise failure propagation, timeout/cancellation behavior, and bounded overload handling.

## Scenarios

The private downstream service receives its scenario through Compose environment variables, not through the client-facing request:

- `success`: wait for the requested delay and return `200`.
- `error`: return `503` without waiting.
- `slow`: wait for the requested delay plus a fixed 1,000 ms, exceeding the SUT downstream timeout.
- `capacity`: admit at most `DOWNSTREAM_MAX_IN_FLIGHT` requests; reject excess requests immediately with `503`.

The SUT public contract remains `/downstream?delay=50`. SUTs map downstream failures and timeouts to `502`, while caller cancellation remains cancellation and does not produce a success response.

## Harness

The existing runner will record `DOWNSTREAM_MODE` and `DOWNSTREAM_MAX_IN_FLIGHT` in `run.json`. The matrix runner will accept `SCENARIOS`; non-success scenarios run only against `/downstream`, while the control `/io` remains a success-only baseline. The downstream service is started with the SUT and its metrics remain separate from SUT measurements.

## Testing and limits

Unit tests will cover each downstream mode, capacity rejection, and existing success behavior. Integration smoke tests will validate error and slow mappings. Short matrix runs will verify artifact and cleanup behavior. This increment will not claim recovery from a dynamic dependency transition; recovery sequencing remains a later phase.
