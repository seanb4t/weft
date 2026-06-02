<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-re2; do not edit manually; use `/adr update weft-re2` -->

# Use an injectable Runner interface for all subprocess calls

**Date:** 2026-06-02
**Status:** Accepted
**Decision:** weft-re2
**Deciders:** Sean Brandt

## Context

The weft engine shells out to bd and jj for every verb (seam 1). Verbs need unit tests that do not exec real subprocesses, which requires the subprocess invocation to be injectable. Plan 1 introduces internal/run.Runner as the single seam between verb logic and os/exec.

## Decision

All subprocess calls go through an injectable run.Runner interface; run.Exec is the real implementation and a recording fake (scriptedRunner) is the test double. Verbs receive the runner via an App struct threaded through the cobra command constructors. A non-zero subprocess exit is represented in Result.Code, never as a Go error.

## Rationale

- Verbs must be unit-testable without real bd/jj installed; the fake records name+args so tests assert the exact subprocess invocation.
- A single one-method interface is the minimal stable contract and does not grow as verbs are added.
- Representing non-zero exit in Result.Code (not a Go error) separates 'command could not start' from 'command ran and reported failure' — enforced uniformly by the interface.

## Alternatives Considered

- Injectable Runner interface (chosen): every verb unit-testable with a recording fake; small stable one-method contract. Cost: App threaded through constructors.
- Direct os/exec in verbs, integration tests only: less boilerplate, but tests need real bd/jj binaries, CI becomes environment-sensitive, and arg-construction bugs go uncaught.
- Functional-option / context-keyed runner: avoids threading App, but obscures the dependency (context is for cancellation, not DI) and is harder for new contributors to discover.

## Consequences

Positive: every future verb is unit-testable with zero external deps; subprocess arg-construction bugs caught at the unit level; the fake doubles as living documentation of the args each verb emits.
Negative: the App struct must be passed to every command constructor (ceremony); the fake must be kept honest — it does not validate that real bd/jj accept the recorded args.
Neutral: standard Go dependency injection, immediately recognizable; integration tests against real bd/jj remain complementary, not the primary gate.
