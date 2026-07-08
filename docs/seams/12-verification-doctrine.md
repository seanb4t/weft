<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 12 — Verification doctrine: no self-certification across real boundaries

> Status: **designed** (design-reviewer READY, round 2; round-1 findings
> fixed inline). Not yet built. Sub-spec of
> [`docs/design.md`](../design.md). Tracked as bead `weft-y85`. Surfaced by the
> first weft-weaves-weft self-weave post-mortem (`weft-rcc` / PR #115), where
> `weft status` shipped green through every gate yet non-functional
> (`weft-1ve`). Decisions resolved with Sean 2026-07-07; the six decision
> records live as `bd note`s on `weft-y85` and in §7 below.

## 1. Scope

Weft's promise is "point me at a spec and I deliver **working** software in a
codebase I've never seen." Today it can deliver green-but-hollow software: one
executor, in one context, authors the test, the fake it drives, and the
production code; the gate runs the suite and sees green — but green only proves
the three artifacts agree with each other, which is guaranteed because one mind
wrote them to agree. This seam redesigns the **planning + verification
doctrine** so that circular self-certification is structurally impossible for
substrate-touching work — generally, across languages and repos.

The framing lens, fixed in the design session: *someone uses weft tomorrow to
deliver a new project or a feature in a codebase we've never seen, in a
language that isn't Go.* Nothing in the doctrine may assume Go, `go test`, bd,
or jj **in the target repo**. Weft's own engine mechanics (bead labels, jj
changes) are its substrate and remain; the target repo's language, framework,
and test tooling enter only through the per-project verification profile (§3).

In scope: the doctrine invariant (§2); the verification profile (§3); the
validation-surface taxonomy and planner rules (§4); the verify gate's three
questions and their enforcement at seal (§5); the waiver escape valve (§6);
decision records (§7); impact map (§8); acceptance (§9).

Out of scope (other seams / later work): the engine command surface itself
(seam 1); plan emission mechanics (seam 2); the reviewer agent's bug/security
methodology (unchanged); any per-project rigor dial (rejected, §7 D5); CI
enforcement of the doctrine outside weft's loop.

## 2. The invariant

> **Green must come from outside the circle.** For any pick whose change
> crosses a real boundary, at least one piece of passing evidence must
> originate from something the executor did not author in the same context:
> a real instance of the dependency, the actual built artifact, or an
> independent adjudicator. Consistency among executor-authored artifacts
> (code + fakes + tests) is never accepted as proof of behavior.

Two corollaries drive all mechanics below:

- **The break must be structural, not disciplinary.** Skill prose asking
  executors to "write real integration tests" is the pre-seam-12 state; the
  post-mortem shows it does not survive contact with an executor's mistaken
  assumption. The gate must be unable to pass without external evidence.
- **The cost must be proportionate.** For pure-logic work the executor's own
  tests *are* the subject, and fakes are legitimate. The taxonomy (§4) is the
  only throttle: rigor scales with what the change touches, never with a
  config knob.

## 3. The verification profile

The profile is the language-agnostic bridge: weft cannot hardcode `go test`,
`pytest`, or `cargo test`, so each managed repo carries a discovered (or
established) contract answering *how this repo proves software works*.

### 3.1 Content and home (split by consumer and trust model)

**Machine-executed commands** live in `.weft/config.toml [verify]` — already
the engine's trusted-config `sh -c` boundary (see the SECURITY note in
`internal/config/config.go`):

```toml
[verify]
# Tier keys — executed by the engine (weft pick verify), one opaque command each:
unit        = "task test:unit"              # fast; fakes legitimate
integration = "task test:integration"       # exercises real boundaries (see conventions)
e2e         = "task test:e2e"               # the repo's own smoke entrypoint (see below)
command     = "task test"                   # legacy key — treated as `unit` (back-compat)
# Auxiliary keys — consumed by AGENTS (executor/reviewer/harness picks), never
# executed by the engine:
build       = "task build"                  # produce the artifact
run         = "./dist/app"                  # launch it (CLI / server)
```

- **Tier keys vs auxiliary keys.** The engine executes exactly three keys —
  `unit`, `integration`, `e2e` — each as **one opaque command** via `sh -c`.
  `build` and `run` are *agent-consumed*: the executor uses them to drive the
  artifact during work, the reviewer during adjudication, and a
  harness-establishment pick when authoring the `e2e` entrypoint. The engine
  never sequences `build` → `run` → observe itself — per-project process
  lifecycle (launching servers, tearing down containers) stays inside the
  repo's own `e2e` command, keeping the engine language-agnostic and dumb.
  The `weft-4e8` requirement ("build the binary, run its actual command
  against real state") is implemented *inside* the target repo's `e2e`
  command — for weft itself, a script that builds and drives a scratch bd.
- Any **tier key** may be absent. An absent tier is an **explicit gap**:
  recorded, reported by verify (§5.1), and never silently equated to "unit is
  enough." Absent auxiliary keys are not gaps; they only limit what agents
  can do without asking.
- `command` remains valid and aliases the `unit` tier, so existing repos keep
  working unchanged until re-onboarded.
- Accessors follow the sibling-consistency rule established by
  `LivenessThreshold`/`MaxResolveAttempts`: degenerate values (empty strings,
  whitespace) are rejected uniformly, not clamped.

**Prose conventions** live in bd memories (surfaced by `bd prime`): the test
framework and idiom, how this repo stands up a real dependency (testcontainers,
scratch DB, temp dir, fixture server), test layout norms. Planner and reviewer
read these to plan and judge tests *in idiom*; the engine never parses them.

### 3.2 Provenance

- **`onboard` discovers, the human approves.** Discovery proposes the
  `[verify]` block; because these strings are executed via `sh -c`, an
  agent-discovered command never lands in trusted config silently — the human
  approves the config write during onboarding. Conventions memories are
  written by onboard directly (they are prose, not executable).
- **`new-project` establishes.** Greenfield scaffolding includes the test
  harness and writes the profile as part of project birth — a weft-born repo
  never has a gap unless the human explicitly accepts one.
- onboard flags loudly when a repo has **no integration/e2e story**; the gap
  rules (§4.3) take over from there. onboard's scope stays "map + seed" — it
  does not scaffold harnesses (§7 D4).

## 4. Validation surfaces and planner rules

### 4.1 The taxonomy

Every pick carries exactly one validation surface, declared by the planner in
`warp-plan.json` (required `surface` field per pick) and materialized by
`plan emit` as a `surface:<value>` bead label. `weft plan check` **fails** a
plan containing a pick without a declared surface.

| Surface | Meaning | Required tiers (cumulative) |
|---|---|---|
| `pure-logic` | algorithm / transform; the logic is the subject | `unit` |
| `integration` | crosses a real boundary: DB, filesystem, subprocess/CLI it shells out to, another module's real contract | `unit` + `integration` |
| `e2e` | user-observable: a command, an endpoint, a UI flow | `unit` + `integration` + `e2e` |

The ladder is cumulative: an `e2e` pick owes all three tiers. Fakes are
legitimate only where the table says `unit` alone.

### 4.2 Planner rules (additions to `planner.md`)

1. **Code and its tests are one pick.** Tests are never split into a trailing
   (droppable) pick. The existing per-pick TDD heuristic survives inside this
   rule.
2. **Acceptance is observable real behavior** for `integration`/`e2e` picks —
   phrased so a fixture cannot satisfy it by construction ("on a project with
   closed items, `status` lists them under *done*"), and stated against the
   real substrate the profile names. This connects Goal-Backward's observable
   truths (planner §Methodology 1) to per-pick acceptance mechanics — the
   truths stop gating only *which picks exist* and start gating *how each pick
   is proven*.
3. **Surface honesty over optimism.** When in doubt between two surfaces,
   declare the stricter one; verify's re-derivation (§5.3) will catch
   under-declaration, but a caught mismatch is a logged defect in planning.

### 4.3 Gap rule — lazy scaffold at first need

Nothing blocks at onboard time. The first plan that contains an
`integration`/`e2e` pick against a profile with a gap in a required tier
forces the planner to do exactly one of:

- **(a)** include a harness-establishment pick (create the missing tier's
  story: the test harness, the scratch-dependency mechanism, the profile
  entry) as a `needs` dependency of the affected picks; or
- **(b)** attach a planning-time waiver (§6) to each affected pick.

There is no third option; `plan check` rejects a plan whose required tiers are
neither present in the profile nor covered by (a)/(b). Silent unit-only
fallback is structurally unrepresentable.

Note the contract change this implies: today's `plan.Validate` is pure over
the `WarpPlan` alone; the gap rule makes plan validation additionally consult
the loaded `[verify]` profile. The CLI layer (which already loads config)
passes the profile in — validation stays deterministic and mutation-free, but
is no longer a function of the plan document only.

## 5. The gate — three questions, enforced at seal

Real verification asks, in order:

1. **Do the right tests exist?** — against the pick's surface, are the
   required test kinds present?
2. **Do they run and pass?** — actually execute the required tiers per the
   profile, against real dependencies where required.
3. **Do the tests actually validate the behavior?** — are they circular
   (self-authored fixtures asserting themselves)? Does the integration test
   touch the real thing? Would *any* test go red if the feature were broken?

Question 2 is deterministic (engine). Questions 1 and 3 are judgment
(reviewer agent). The pre-seam-12 gate answered only a weak version of #2.

### 5.1 Q2 — engine: surface-aware tier execution

`weft pick verify <bead>` reads the bead's `surface:` label and runs the
required tiers from the profile in ladder order, failing fast. The envelope
grows per-tier results:

```json
{"ok": true, "verb": "pick.verify",
 "data": {"pass": false, "bead": "…", "change": "…",
          "surface": "integration",
          "tiers": {"unit": "pass", "integration": "fail"},
          "reason": "integration tier failed: …"}}
```

- `pass` requires every required tier green. Exit-code semantics are
  unchanged: exit 0 = engine ran; the verdict is data. (`data.change` is new
  to the envelope alongside the tier fields — the current implementation
  emits only `bead`/`pass`; the impact map carries the addition.)
- A required tier whose profile key is **absent** reports `"gap"` and forces
  `pass: false` unless the bead carries a `verify-waiver:<tier>` label for
  that tier (§6) — the engine-level enforcement of §4.3. The waiver's tier
  coverage is the **label value itself** (machine-checkable, consistent with
  every other engine-deterministic check keying off bd labels); the engine
  never parses note prose.
- A pick with no `surface:` label fails verify as an invocation error: the
  plan predates seam 12 or bypassed `plan check`. There is no implicit
  default surface. Migration for in-flight beads when this ships mid-wave:
  `bd update <bead> --add-label surface:<value>` (choosing honestly per
  §4.2 rule 3); re-emitted plans get labels automatically.

### 5.2 Q1 + Q3 — reviewer: mandatory adequacy adjudication

For `integration` and `e2e` picks, the orchestrator (execute skill)
**mandatorily** dispatches `weft-reviewer` before seal — it is no longer
optional (`review:deep` remains as an opt-in for `pure-logic` picks).
`reviewer.md` gains an **adequacy** section answering, as structured findings:

- **Existence (Q1):** does at least one test exercise the real boundary /
  drive the built artifact, per the surface? An integration pick with only
  mocked tests FAILS here regardless of Q2 green.
- **Validity (Q3):** are the tests circular? Does the integration test touch
  the real thing named by the profile conventions? Mutation mindset: would
  any test go red if the feature were broken? Is the pick's
  observable-behavior acceptance actually exercised?
- **Waiver adjudication (§6):** where a waiver exists, was the declared
  substitute validation actually performed?

An adequacy fail is a BLOCKER-severity finding: it sets `pass: false` in the
reviewer's envelope and blocks seal. This resolves the seam 5 §8 deferred
question — the reviewer stays a **separate agent** (fresh context is precisely
what makes it an independent adjudicator); the engine gate keys off its
recorded verdict rather than absorbing it.

### 5.3 Surface reconciliation — declare + re-derive, stricter wins

The planner *declares* the surface (§4.1) so the executor knows the evidence
bar before working. At adjudication time the reviewer independently
**re-derives** the surface from the actual diff — did the change add a CLI
verb? import a real client? spawn a subprocess? touch the filesystem? — and
reconciles:

- **Mismatch → the stricter surface wins, automatically.** The orchestrator
  updates the bead's `surface:` label, logs the discrepancy as a finding, and
  verify/seal proceed against the stricter bar. No human stall.
- A misclassification (or mid-execution scope drift) can therefore *raise*
  the bar but never silently lower it — one wrong label cannot disable the
  teeth. This is the LivenessThreshold sibling-consistency lesson applied to
  classification.

### 5.4 Enforcement point — `weft pick seal`

After the reviewer passes adequacy, the **orchestrator** records the verdict
on the bead: an `adequacy:pass` label plus a `bd note` carrying the findings
summary and reviewer-run metadata. Then:

- `weft pick seal <bead>` for a pick whose surface is `integration`/`e2e`
  **refuses to seal** unless the bead carries `adequacy:pass`. A waiver (§6)
  never bypasses adjudication — it changes what adequacy adjudicates (the
  declared substitute evidence instead of the waived tier); the reviewer still
  issues the verdict. `pure-logic` picks seal on Q2 alone.
- The executor still runs `weft pick verify` locally for fast feedback during
  work, but cannot self-certify: for real-boundary picks, seal is mechanically
  gated on evidence the executor cannot produce inside its own context.

**Known limit (stated, not hidden):** bd labels have no ACL; a hostile
executor could forge `adequacy:pass`. The engine check is the structural
backstop against the *accidental* self-certification that actually occurred;
label provenance is orchestration discipline, and the bd audit trail (notes,
timestamps) makes forgery detectable rather than impossible. Hardening label
provenance is out of scope for this seam.

## 6. The escape valve — waivers

When real-boundary validation is genuinely infeasible (paid API, prod-only
infrastructure, sandbox limits), the pick may carry an explicit, auditable
waiver: one `verify-waiver:<tier>` label **per waived tier** (e.g.
`verify-waiver:integration`) — the tier name in the label value is what the
engine checks (§5.1) — plus a structured `bd note` stating *what was
infeasible, why, and what validation substitutes* ("validated by X instead").
Two creation paths, both crossing a human:

- **Planning-time** (infeasibility known up front): the planner authors the
  waiver in `warp-plan.json`; it materializes at emit and is visible in the
  `plan emit --dry-run` output the human approves. Approving the plan approves
  the waiver.
- **Execution-time** (infeasibility discovered mid-pick): the executor raises
  a **checkpoint** (the existing blocking-deviation protocol); the human
  routes — grant the waiver, replan, or redo.

Invariants: **no agent self-grants a waiver**; a waiver names the tier(s) it
covers and the substitute evidence; the reviewer's adequacy pass (§5.2)
verifies the substitute was actually performed. A waiver is a logged,
reviewable decision — never a silent fallback to mocks.

## 7. Decision record (resolved with Sean, 2026-07-07)

| # | Decision | Resolution |
|---|---|---|
| D1 | Gate shape: how deterministic results and adequacy verdict combine; hardness of adequacy fail | **Hard fuse, enforced at seal.** Both required for `integration`/`e2e`; adequacy fail blocks seal; `pure-logic` stays command-only. Reviewer stays a separate agent (resolves seam 5 §8). |
| D2 | Surface classification ownership + misclassification catch | **Planner declares + reviewer re-derives; stricter wins automatically**, discrepancy logged. No human stall. |
| D3 | Escape valve | **Hybrid, human-visible on both paths**: planner-authored waivers ride the dry-run gate; executor-discovered infeasibility uses the checkpoint protocol. No agent self-grants. |
| D4 | Missing integration/e2e story in an unfamiliar repo | **Lazy scaffold at first need**: onboard records gaps loudly; the first affected plan must add a harness pick or a waiver; `plan check` enforces. onboard keeps map+seed scope. |
| D5 | Speed vs. rigor | **Taxonomy only.** No rigor knob (a down-dial recreates the hole; an up-only ratchet is addable later without doctrine change). |
| D6 | Profile home (found in grounding; the bead left it ambiguous) | **Split**: commands → `.weft/config.toml [verify]` (trusted-config boundary; onboard proposes, human approves); conventions → bd memories. |

## 8. Impact map

| Artifact | Change |
|---|---|
| `internal/config` | `[verify]` schema: tier keys (`unit`/`integration`/`e2e`; `command` aliases `unit`) + agent-consumed auxiliary keys (`build`/`run`), gap-aware accessors with uniform degenerate-value guards |
| `internal/cli/pick.go` — `verify` | surface-aware tier execution (tier keys only — `build`/`run` are agent-consumed, §3.1); per-tier envelope (`data.surface`, `data.tiers`, plus the previously missing `data.change`); gap ⇒ fail unless `verify-waiver:<tier>` label present; unlabeled pick ⇒ invocation error |
| `internal/cli/pick.go` — `seal` | adequacy/waiver gate for `integration`/`e2e` surfaces |
| `internal/plan` — `check`/`emit` | required per-pick `surface` field; §4.3 gap rule validation (`Validate` gains the loaded `[verify]` profile as input — no longer plan-document-pure); `surface:` label + `verify-waiver:<tier>` label emission |
| `plugin/agents/planner.md` | §4 taxonomy + rules; waiver authoring; gap obligations |
| `plugin/agents/reviewer.md` | adequacy section (Q1+Q3), surface re-derivation, waiver adjudication; drop the seam 5 §8 deferral note |
| `plugin/agents/executor.md` + `plugin/skills/execute` | mandatory reviewer dispatch for real-boundary picks; adequacy-label recording; checkpoint waiver path; executor-cannot-self-certify choreography |
| `plugin/skills/onboard` | profile discovery + propose/approve flow; conventions memories; gap flagging |
| `plugin/skills/new-project` | profile establishment; harness scaffolding at birth |
| `plugin/references/tdd-verify-discipline.md` | rewritten around the invariant + three questions; fixes stale `weft.yaml`/`tdd_mode` references (the engine reads `.weft/config.toml`; no `tdd_mode` exists) |

## 9. Acceptance — what proves the seam

- **`weft-4e8` folds in** as the `e2e` tier of Q2 (§5.1): "verify runs the
  smoke command against real state" is that seam's concrete first pick.
- **`weft-1ve` is the exemplar.** Re-run the `--all` bug scenario under the
  new gate and show it is caught **twice**: Q2's integration tier fails (a
  real scratch bd shows closed items missing from `status` output), and
  Q1/Q3 adequacy fails (an integration-surface pick presented only
  fake-driven tests). The doctrine's acceptance criterion is: *this class of
  defect cannot ship green again.*
- The seam is ready to plan into the warp when the design-reviewer returns
  READY; implementation decomposition (pick boundaries, wave order) belongs
  to the planning pass, not this spec.
