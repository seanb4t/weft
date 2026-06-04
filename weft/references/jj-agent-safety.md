<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# jj agent-safety profile

Every weft agent and workflow that touches VCS follows this profile without
exception. It is a hard contract, not a style guide.

## Rules

### 1. Always pass `--no-pager`

Every `jj` invocation MUST include `--no-pager`. Without it, jj may open an
interactive pager, blocking a non-interactive agent process indefinitely.

```bash
# correct
jj --no-pager log -r @

# wrong — may block
jj log -r @
```

### 2. Use `--git` on diffs

When inspecting diffs, pass `--git` to get a stable, machine-readable unified
diff format.

```bash
jj --no-pager diff --git
jj --no-pager show --git -r <change-id>
```

### 3. Reference change-ids, not commit hashes

jj change-ids (e.g. `sqpuoqvx`) are stable across rebase and history
rewriting. Git commit SHAs are ephemeral in a jj repo and MUST NOT be used
as canonical references. Always use the change-id form when bookmarking,
labeling, or communicating a revision.

### 4. Edit conflict markers directly — never `jj resolve`

`jj resolve` launches an interactive merge tool and hangs a non-interactive
agent. When conflicts arise, agents MUST edit the conflict marker blocks in
the file directly, then verify with `jj --no-pager st` that the conflict
count drops to zero.

### 5. Always pass `-m` — never open an editor

`jj commit` and `jj describe` MUST always receive the message via `-m "..."`.
Never call them without `-m`; the editor invocation blocks a non-interactive
agent.

```bash
jj --no-pager commit -m "feat(scope): description"
```

### 6. Fetch at task start

Run `jj --no-pager git fetch` before beginning any task that creates or
modifies changes. This ensures the local op log is current and avoids
divergence from the remote.

### 7. Recovery is change-scoped, never op-restore

| Situation | Correct recovery |
|-----------|-----------------|
| Bad changes to working copy | `jj --no-pager abandon <change-id>` |
| Bad operation on the op log | `jj --no-pager op revert <op-id>` |
| Global history rewind | **BLOCKED** — `jj op restore` is human-gated only |

`jj op restore` rewinds the global operation log and stales every other
workspace sharing the repo. It MUST NOT be invoked by an agent. Use
change-scoped abandonment or op-revert instead.

## Engine verb boundary

The weft engine verbs (`weft shed`, `weft pick`, `weft conflict`) already
encapsulate the dangerous multi-step jj choreography — fetch, new, seal,
land, abandon on failure. Agents MUST call the verb, not raw jj, except
when directly editing files or conflict markers inside an active change.
