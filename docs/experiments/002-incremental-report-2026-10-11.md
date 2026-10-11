# Experiment 002 Incremental Report — 2026-10-11

## Scope and baseline

Experiment 002 starts from the completed Experiment 001 source and harness. Experiment 001 remains unchanged. New measurements must use a fresh external root such as `/tmp/sut-exp002-<timestamp>` and must retain raw Vegeta streams, Docker stats, `run.json`, summaries, and selected profiles.

## Implementation status

- [x] Separate Experiment 002 directory created from the Experiment 001 reproducible baseline.
- [x] Java client variants are explicit: `sync`, `async`, and `sync-tuned`.
- [x] Go baseline is recorded as `go-net-http`.
- [x] CPU allocation is configurable as `SUT_CPUS=1.0`, `2.0`, or `4.0` and recorded in metadata.
- [x] Summaries classify capacity responses, application failures, and transport/timeout failures separately.
- [x] Full repeated primary matrix at three repetitions per point for branches 1/4/8 at 800/s, for Java `sync`, Java `async`, and Go.
- [x] Branch-cap confirmation at 128, 256, and 512 (one repetition each).
- [x] CPU scaling confirmation at one, two, and four effective CPUs (one repetition each, branches=4).
- [x] Selected Java JFR and Go CPU/heap/goroutine/trace captures.
- [ ] Final artifact-derived report and cautious conclusions.

## Captured external artifacts

- Corrected sync pilot: `/tmp/sut-exp002-pilot-sync-valid-20261011`
- Corrected async pilot: `/tmp/sut-exp002-pilot-async-valid-20261011`
- Three-repetition sync primary: `/tmp/sut-exp002-primary-sync-20261011`
- Three-repetition async primary: `/tmp/sut-exp002-primary-async-20261011`
- CPU confirmation: `/tmp/sut-exp002-cpu-20261011`
- Branch-cap confirmation: `/tmp/sut-exp002-branchcaps-20261011`
- Java profile: `/tmp/sut-exp002-profile-java-20261011`
- Go profile: `/tmp/sut-exp002-profile-go-20261011`

The first pilot attempt, `/tmp/sut-exp002-pilot-sync-20261011`, used malformed escaped query separators and was stopped before completion. It is retained as an invalid harness-input attempt and is excluded from all reports. A parallel profile attempt was also invalidated by a Compose container-name conflict and was not used.

The primary roots contain 18 summaries each: three workloads, two services, and three repetitions. The derived summaries distinguish intentional `503` capacity responses from transport/timeout failures. Those roots include capacity rejection and a small number of transport/timeout failures at the one-second measurement duration; they require cautious interpretation and do not establish a transport cause.

## Interpretation boundary

Until the unchecked measurements are complete, this report makes no performance or causal claim. `sendAsync` completion, connection reuse, response-body handling, allocation, GC, CPU saturation, permit exhaustion, and downstream capacity remain competing explanations to be tested with repeated controlled observations.
