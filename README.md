# Systems Under Test

A repository for reproducible technical experiments across software systems, runtimes, architectures, tools, and workloads.

Each experiment is self-contained. Its directory contains the implementation, setup, workload definitions, tests, measurement harnesses, and experiment-specific README. The root repository provides shared organization and documentation conventions; it does not assume that every experiment has the same technology or research topic.

## Repository organization

```text
docs/                         Cross-experiment plans and supporting documents
experiments/<experiment>/     One self-contained experiment per directory
```

An experiment may contain any of the following, as appropriate:

```text
README.md                     Experiment-specific scope, setup, and findings
requirements or design docs   Questions, hypotheses, controls, and acceptance criteria
source directories            Systems, services, tools, or configurations under test
workloads/                    Inputs, scenarios, fixtures, and load definitions
scripts/                      Reproduction, validation, measurement, and analysis tools
infrastructure/               Containers, dependencies, and environment setup
analysis/                     Derived analysis code and report templates
results/                      Lightweight placeholders; large raw artifacts stay external
```

## Experiment catalog

- [Experiment 001 Java Virtual Threads and Go Goroutines](experiments/001-virtual-threads-vs-goroutines/README.md)
- [Experiment 002 Java HTTP Client and CPU Scaling](experiments/002-http-client-and-scaling/README.md)

Each experiment README is the authoritative entry point for that experiment. It should explain what is being tested, how to run it, what versions and controls are required, where artifacts are written, and how conclusions should be interpreted.

## How to use this repository

1. Choose an experiment under `experiments/`.
2. Read that experiment’s README and requirements or design documents.
3. Install the tools listed by the experiment.
4. Run the documented validation or smoke checks before measurement.
5. Use a fresh external directory for raw results unless the experiment explicitly says otherwise.
6. Preserve source revisions, tool versions, environment details, workload inputs, and configuration with every measured run.
7. Read the experiment’s analysis and interpretation notes before drawing conclusions.

There is intentionally no single repository-wide benchmark command. Different experiments may require different languages, infrastructure, data, hardware, or measurement methods.

## Reproducibility principles

Experiments should make their variables and limits visible. Where practical, they should:

- Pin relevant tool and dependency versions.
- Separate control workloads from the behavior under study.
- Define comparable configurations before collecting results.
- Use tests before changing experimental implementations.
- Preserve raw inputs and measurement artifacts without editing them.
- Record source identity, environment, timing, and configuration metadata.
- Repeat important measurements and report variability.
- Distinguish observed results from explanations and hypotheses.
- State what the experiment does not establish.

Large profiles, generated reports, container statistics, and host-specific benchmark output should normally remain outside Git. The corresponding experiment documentation should describe how to reproduce them and how the raw artifacts are organized.

## Adding an experiment

Create a new directory under `experiments/` with a focused name and add an experiment-specific `README.md` before collecting results. At minimum, document:

- Purpose and research questions.
- Hypotheses or expected observations.
- Systems and versions under test.
- Workload and control variables.
- Environment and fairness requirements.
- Test and validation commands.
- Measurement protocol and artifact format.
- Analysis method and acceptance criteria.
- Interpretation boundaries and non-goals.

Cross-experiment planning material can live under `docs/`, but experiment-specific instructions and conclusions belong with the experiment they describe.
