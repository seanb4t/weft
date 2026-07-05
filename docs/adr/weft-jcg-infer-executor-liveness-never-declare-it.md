---
title: "Infer executor liveness; never declare it"
---
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-jcg; do not edit manually; use `/adr update weft-jcg` -->

**Date:** 2026-07-04
**Status:** Accepted
**Decision:** weft-jcg
**Deciders:** Sean Brandt

## Context

Three finished-or-nearly-finished picks sat stranded in workspaces for 11 days, invisible to `bd ready` and `bd blocked`, because nothing distinguished a crashed executor from a thinking one. `weft reap` and the new `weft doctor` both need a liveness signal, but the engine never spawns the agent process (design.md §7) and so cannot own a PID.

## Decision

The engine infers liveness by joining a workspace's jj working-copy committer timestamp (`jj log -r '<name>@'` — refreshed by jj's per-workspace snapshot on every command run there) with a max-mtime walk of the workspace directory (excluding `.jj/`) against a configurable `[liveness] threshold` (default 45m). It never introduces a PID file or heartbeat protocol.

## Rationale

- weft never spawns the agent process, so it cannot own or verify a PID.
- A written heartbeat/PID file requires agent cooperation — exactly the works-when-watched fragility roadmap §3 targets.
- Existing jj snapshot + mtime state already exists from any real work; no new coordination needed. Empirically validated 2026-07-04: an idle workspace's `@` timestamp was 12 days old while an active one's was minutes old.

## Alternatives Considered

- **Recency inference — jj commit timestamp + mtime walk (chosen):** needs no new signal or agent cooperation; uses state real work already produces.
- **PID file at dispatch (rejected):** immediate, explicit signal, but the engine never spawns agents so it cannot own or verify the PID; requires an agent-written protocol and PID-reuse guarding.
- **Heartbeat file (rejected):** fresh, dedicated signal, but requires cooperative writes from every future agent/host — the fragility this milestone removes.

## Consequences

- Positive: reap and doctor gain a usable liveness signal with zero new agent-side contract.
- Negative: a conservative threshold can misclassify a quiet-but-alive executor as dead (bounded: reap runs at orchestrator startup/resume, not mid-wave; jj snapshots mean sealed work survives reaping).
- Neutral: future non-Claude hosts inherit the same no-signal-needed contract.
