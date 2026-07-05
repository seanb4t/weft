// internal/cli/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/plan"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newPlanCmd() *cobra.Command {
	p := &cobra.Command{Use: "plan", Short: "Planning -> warp emission (spec seam 2)"}
	p.AddCommand(a.newPlanCheckCmd(), a.newPlanEmitCmd())
	return p
}

func (a *App) newPlanEmitCmd() *cobra.Command {
	var dryRun bool
	var epic string
	var allowDrop bool
	c := &cobra.Command{
		Use:   "emit <file>",
		Short: "Emit the warp from warp-plan.json (derive edges, preview, create/upsert)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			if issues := plan.Validate(wp); len(issues) > 0 {
				return exit.Invocationf("warp-plan is invalid (%d issue(s)); run 'weft plan check' first", len(issues))
			}
			d := plan.Derive(wp.Picks, a.Config.PlanStructural(), a.Config.PlanOverlapMax())
			if epic != "" {
				// Standard epic-id allowlist guard (mirrors finish.go / the
				// changeIDPattern revset-injection idiom): rejects leading dashes,
				// revset metacharacters, and path-walk sequences before the value
				// is forwarded to a subprocess or interpolated into a revset.
				if err := validateEpicID(epic); err != nil {
					return err
				}
				// --allow-drop is a first-emit-only flag; reject it early on the replan path.
				if allowDrop {
					return exit.Invocationf("--allow-drop is not supported with --epic (replan): the bd import path has no field-drop preflight, so the flag would be a silent no-op")
				}
				return a.planReplan(cmd, wp, d, epic, dryRun)
			}
			return a.planFirstEmit(cmd, wp, d, dryRun, allowDrop)
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "preview the warp without mutating beads")
	c.Flags().StringVar(&epic, "epic", "", "existing epic id to re-plan against (bd import upsert)")
	c.Flags().BoolVar(&allowDrop, "allow-drop", false, "proceed despite bd dropping unknown graph fields (loud, opt-in; first emit only; not valid with --epic)")
	return c
}

// planFirstEmit creates a brand-new warp via bd create --graph (spec §5),
// gated by a bd-backed dry-run preflight that refuses to silently drop fields
// (seam 9 / docs/seams/09-emit-field-drop-guard.md).
func (a *App) planFirstEmit(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, dryRun, allowDrop bool) error {
	isRoadmap := len(wp.Phases) > 0

	graph, err := plan.GraphJSON(wp, d)
	if err != nil {
		return err
	}
	path, cleanup, err := writeTempPayload("weft-warp-*.json", graph)
	if err != nil {
		return err
	}
	defer cleanup()

	// Preflight: bd's own dry-run reports dropped fields (stderr) + the parsed
	// graph shape (stdout). It mutates nothing, so we can abort before any create.
	pre, err := run.BD(a.Runner, "create", "--graph", path, "--dry-run", "--json")
	if err != nil {
		return exit.Hardf("bd create --graph dry-run could not run: %v", err)
	}
	if pre.Code != 0 {
		return exit.Hardf("bd create --graph dry-run failed: %s", strings.TrimSpace(pre.Stderr))
	}
	pf, err := plan.ParsePreflight([]byte(pre.Stdout), []byte(pre.Stderr))
	if err != nil {
		return exit.Hardf("%v", err)
	}
	wantNodes, wantEdges := 1+len(wp.Picks), len(d.Edges)
	if isRoadmap {
		// Roadmap path: phase edges come from authored needs inside GraphJSON,
		// not from Derive — d.Edges is always empty here and must not be used.
		wantNodes, wantEdges = plan.RoadmapCounts(wp)
	}
	issues := plan.CheckPreflight(pf, wantNodes, wantEdges)

	warnings := []string{}
	if issues.CountMismatch != "" {
		return exit.Hardf("plan emit aborted: %s", issues.CountMismatch)
	}
	if len(issues.Drops) > 0 {
		if !allowDrop {
			return exit.Hardf("plan emit aborted — bd would drop fields (data loss); fix the payload or pass --allow-drop:\n%s",
				strings.Join(issues.Drops, "\n"))
		}
		warnings = append(warnings, issues.Drops...)
	}
	warnings = append(warnings, pf.Notes...)
	if issues.SchemaNote != "" {
		warnings = append(warnings, issues.SchemaNote)
	}

	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "create", "epic": wp.Epic.Title,
			"edges": d.Edges, "tolerated": d.Tolerated,
			"schema_version": pf.SchemaVersion, "warnings": warnings,
		}
		if isRoadmap {
			data["phases"] = len(wp.Phases)
		} else {
			data["picks"] = len(wp.Picks)
		}
		return Emit(cmd, "plan.emit", data, planPreviewText("create", wp, d))
	}

	res, err := run.BD(a.Runner, "create", "--graph", path, "--json")
	if err != nil {
		return exit.Hardf("bd create --graph could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd create --graph failed: %s", strings.TrimSpace(res.Stderr))
	}
	// Parse the ids map (node key -> created bead id). Shape verified live:
	// {"ids":{"@epic":"...","<ref>":"..."},"schema_version":1}. The warp was
	// already created at this point, so a parse failure is a hard error (loud,
	// never a silently degraded envelope) — the operator must investigate.
	var applied struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &applied); err != nil || len(applied.IDs) == 0 {
		return exit.Hardf("warp created but bd create --graph --json output is unparseable (ids missing) — investigate before re-running (a re-run would duplicate the warp): %v\noutput: %s",
			err, strings.TrimSpace(res.Stdout))
	}
	// Surface any warning the real create emits on success.
	if s := strings.TrimSpace(res.Stderr); s != "" {
		warnings = append(warnings, s)
	}
	created := len(wp.Picks)
	if isRoadmap {
		created = len(wp.Phases)
	}
	data := map[string]any{
		"mode": "create", "created": created, "edges": d.Edges,
		"tolerated": d.Tolerated, "schema_version": pf.SchemaVersion,
		"warnings": warnings, "ids": applied.IDs,
		"bd_output": strings.TrimSpace(res.Stdout),
	}
	if isRoadmap {
		data["phases"] = len(wp.Phases)
	}
	var text string
	if isRoadmap {
		text = fmt.Sprintf("emitted roadmap: %d phase(s)\nepic: %s",
			len(wp.Phases), applied.IDs["@epic"])
	} else {
		text = fmt.Sprintf("emitted warp: %d pick(s), %d edge(s), %d tolerated overlap(s)\nepic: %s",
			len(wp.Picks), len(d.Edges), len(d.Tolerated), applied.IDs["@epic"])
	}
	return Emit(cmd, "plan.emit", data, text)
}

// writeTempPayload stages a payload weft must hand to bd as a file path.
func writeTempPayload(pattern string, payload []byte) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, exit.Hardf("temp payload file: %v", err)
	}
	// os.CreateTemp already opens with 0600, but the payload carries pick
	// titles/descriptions/bead-ids and the system temp dir is world-readable
	// (1777 on Linux), so pin owner-only perms explicitly and defensively.
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, exit.Hardf("secure payload file: %v", err)
	}
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, exit.Hardf("write payload: %v", err)
	}
	// A Close error after a successful Write can signal a truncated/corrupt
	// file; surface it rather than handing bd a bad payload.
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", func() {}, exit.Hardf("close payload: %v", err)
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// planPreviewText renders the dry-run human gate (spec §5): edges + the
// warn+tolerate overlaps the human is approving.
func planPreviewText(mode string, wp plan.WarpPlan, d plan.Derivation) string {
	var b strings.Builder
	if len(wp.Phases) > 0 {
		fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d phase(s) (roadmap)\n", mode, wp.Epic.Title, len(wp.Phases))
	} else {
		fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d pick(s), %d edge(s)\n", mode, wp.Epic.Title, len(wp.Picks), len(d.Edges))
	}
	// d.Edges is empty on the roadmap path; this loop is a no-op there.
	for _, e := range d.Edges {
		fmt.Fprintf(&b, "  edge: %s depends on %s\n", e.From, e.To)
	}
	if len(d.Tolerated) > 0 {
		fmt.Fprintf(&b, "  %d tolerated overlap(s) (same shed; conflict resolved via seam 4):\n", len(d.Tolerated))
		for _, o := range d.Tolerated {
			fmt.Fprintf(&b, "    %s ~ %s share %v\n", o.A, o.B, o.Shared)
		}
	}
	b.WriteString("  (no mutation — re-run without --dry-run to emit)")
	return b.String()
}

// planReplan upserts an existing warp via bd import (spec §7): resolve the
// ref->bead map from the epic's weft-ref labels, build the upsert payload, and
// (unless --dry-run) apply it. Sequence: (1) bd import, (2) parent-child wiring
// for new picks via bd dep add (bd import ignores the JSONL parent field —
// verified 2026-06-10), (3) post-import read-back to verify authored fields
// round-tripped (seam 9 §7), (4) DeferredEdge wiring for edges touching a new
// pick via bd dep add --type blocks (seam-2 §8), (5) removed-pick enactment:
// OPEN removed refs are closed with an audit reason after edge wiring, while a
// live replan that would drop an in_progress or closed pick hard-fails BEFORE
// any bd import (invariant I2, ADR weft-0pq) — woven or landed work is never
// silently dropped. bd supersede is not used: it requires --with <new>, and a
// replan diff carries no successor mapping.
func (a *App) planReplan(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, epic string, dryRun bool) error {
	existing, err := a.warpRefMap(epic)
	if err != nil {
		return err
	}
	rp, err := plan.BuildReplan(wp, d, epic, existing)
	if err != nil {
		return exit.Hardf("build re-plan payload: %v", err)
	}
	// I2 (ADR weft-0pq): a LIVE replan can never silently drop work. The
	// classifier fails CLOSED — only an "open" pick is removable, so any removed
	// ref with a non-open status (in_progress, closed, blocked, hooked, deferred,
	// or any unknown/future state) lands in RemovedBlocked. If any is present,
	// hard-fail BEFORE any bd import — no import, no edge wiring, no close.
	// Dry-run reports both classifications (below) without mutating.
	if !dryRun && len(rp.RemovedBlocked) > 0 {
		parts := make([]string, len(rp.RemovedBlocked))
		for i, ref := range rp.RemovedBlocked {
			parts[i] = fmt.Sprintf("%s (%s)", ref, existing[ref].Status)
		}
		return exit.Hardf("re-plan drops %d pick(s) that are not open [%s] — only open picks are removable; any non-open status (in_progress, closed, blocked, hooked, deferred, pinned, or unknown) blocks a live replan to avoid silently dropping work (I2); express supersede intent against open picks only",
			len(rp.RemovedBlocked), strings.Join(parts, ", "))
	}
	warnings := []string{} // vacuously empty for dry-run (no bd call); present for envelope-shape stability
	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "upsert", "epic": epic,
			"updated": rp.Updated, "created": rp.Created, "removed": rp.Removed,
			"removed_blocked": rp.RemovedBlocked,
			// applied_edges: same key as live; on dry-run these are the edges that WILL be wired post-import.
			"applied_edges": rp.DeferredEdges, "tolerated": d.Tolerated,
			"warnings": warnings,
		}
		return Emit(cmd, "plan.emit", data, replanText(epic, rp, true))
	}
	path, cleanup, err := writeTempPayload("weft-replan-*.jsonl", rp.JSONL())
	if err != nil {
		return err
	}
	defer cleanup()
	res, err := run.BD(a.Runner, "import", path, "--json")
	if err != nil {
		return exit.Hardf("bd import could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd import failed: %s", strings.TrimSpace(res.Stderr))
	}
	if s := strings.TrimSpace(res.Stderr); s != "" {
		warnings = append(warnings, s)
	}
	// Parse the positional ids envelope from bd import --json.
	// bd import --json emits {"created":N,"ids":[...],"schema_version":1} where
	// ids[i] is the bead id for input JSONL line i (both create and update paths).
	// This is load-bearing: if the count diverges the warp is structurally corrupt.
	var importRecord struct {
		Created int      `json:"created"`
		IDs     []string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &importRecord); err != nil {
		return exit.Hardf("bd import --json output could not be parsed: %v — the warp is incomplete; investigate", err)
	}
	if len(importRecord.IDs) != len(rp.Expect) {
		return exit.Hardf("bd import --json returned %d ids but %d records were written; positional contract violated — bd import --json guarantees ids[i] is the bead id for JSONL record i; a count mismatch means bd changed its contract or output was truncated — the warp is incomplete; investigate",
			len(importRecord.IDs), len(rp.Expect))
	}
	// Build refToID from positional zip of rp.Expect[i].Ref with importRecord.IDs[i].
	refToID := map[string]string{}
	for i, ex := range rp.Expect {
		refToID[ex.Ref] = importRecord.IDs[i]
	}
	// bd import ignores the JSONL "parent" field on both create and update paths
	// (verified 2026-06-10), so parentage is wired post-import; this also makes
	// the scoped readback below see the new picks.
	for i, ref := range rp.Created {
		id := refToID[ref]
		if id == "" {
			return exit.Hardf("re-plan applied but bd import returned an empty id for created ref %q (record %d); the pick is orphaned and the warp is incomplete — investigate", ref, i)
		}
		dep, err := run.BD(a.Runner, "dep", "add", id, epic, "--type", "parent-child")
		if err != nil {
			return exit.Hardf("re-plan applied but bd dep add (parent-child) for %s->%s could not run: %v", id, epic, err)
		}
		if dep.Code != 0 {
			return exit.Hardf("re-plan applied but parent-child link for %s->%s could not be wired: %s — the warp is incomplete; investigate", id, epic, strings.TrimSpace(dep.Stderr))
		}
	}
	// Post-import read-back: re-read the epic's children and verify that every
	// authored field round-tripped through bd import (seam 9 §7). Parent-child
	// links were wired above, so the scoped readback now sees the new picks.
	readback, err := a.warpReadback(epic)
	if err != nil {
		return err
	}
	if disc := plan.VerifyReplan(rp.Expect, readback); len(disc) > 0 {
		return exit.Hardf("plan emit replan applied but %d authored field(s) did not round-trip (bd dropped them); the warp is incomplete — investigate:\n%s",
			len(disc), strings.Join(disc, "\n"))
	}
	// Wire blocks edges that touched a new pick (seam-2 §7; formerly the §8 sub-seam,
	// shipped in weft-ccy.5) — bd import cannot forward-reference ids inside a
	// batch, so they are applied post-import from the readback map. Any failure
	// leaves the warp structurally incomplete: hard.
	for i, e := range rp.DeferredEdges {
		from, fok := readback[e.From]
		to, tok := readback[e.To]
		if !fok || !tok || from.ID == "" || to.ID == "" {
			return exit.Hardf("re-plan applied but edge %s->%s could not be resolved post-import (edge %d/%d; %d already wired); the warp is incomplete — investigate", e.From, e.To, i+1, len(rp.DeferredEdges), i)
		}
		dep, err := run.BD(a.Runner, "dep", "add", from.ID, to.ID, "--type", "blocks")
		if err != nil {
			return exit.Hardf("re-plan applied but bd dep add could not run for %s->%s (edge %d/%d; %d already wired): %v", e.From, e.To, i+1, len(rp.DeferredEdges), i, err)
		}
		if dep.Code != 0 {
			return exit.Hardf("re-plan applied but edge %s->%s could not be wired (edge %d/%d; %d already wired): %s — the warp is incomplete; investigate", e.From, e.To, i+1, len(rp.DeferredEdges), i, strings.TrimSpace(dep.Stderr))
		}
	}
	// Enact removed-pick closure (seam 2 §8): every OPEN removed ref is closed
	// with an audit reason, AFTER edge wiring so the warp's structure is settled
	// first. Blocked removals (in_progress/closed) never reach here — the I2 guard
	// above aborts a live replan before any bd import. Ids resolve from the
	// pre-import ref map: removed refs are absent from the plan, so they are not
	// in rp.Expect / the post-import readback zip. A close failure leaves the warp
	// inconsistent (import applied, removal not enacted): hard.
	for i, ref := range rp.Removed {
		id := existing[ref].ID
		if id == "" {
			return exit.Hardf("re-plan applied but removed ref %q (removal %d/%d) has no resolvable bead id to close — the warp is incomplete; investigate", ref, i+1, len(rp.Removed))
		}
		reason := fmt.Sprintf("removed by replan of %s (was %s%s)", epic, plan.RefLabelPrefix, ref)
		cl, err := run.BD(a.Runner, "close", id, "-r", reason)
		if err != nil {
			return exit.Hardf("re-plan applied but bd close for removed pick %s (%s) could not run: %v — the warp is incomplete; investigate", id, ref, err)
		}
		if cl.Code != 0 {
			return exit.Hardf("re-plan applied but bd close for removed pick %s (%s) failed: %s — the warp is incomplete; investigate", id, ref, strings.TrimSpace(cl.Stderr))
		}
	}
	data := map[string]any{
		"mode": "upsert", "epic": epic,
		"updated": rp.Updated, "created": rp.Created, "removed": rp.Removed,
		"removed_blocked": rp.RemovedBlocked,
		"applied_edges":   rp.DeferredEdges, "tolerated": d.Tolerated,
		"bd_output": strings.TrimSpace(res.Stdout), "warnings": warnings,
		"verification": []string{},
	}
	return Emit(cmd, "plan.emit", data, replanText(epic, rp, false))
}

// warpChild is one `bd list --parent <epic> --json` record. It carries the
// superset of fields the warp readers consume (warpRefMap needs id+status;
// warpReadback needs title+priority+description); both always need labels.
type warpChild struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
}

// warpScan reads an epic's children in one bd list call and rebuilds a ref->V
// map by scanning each child's weft-ref:<ref> label (spec §3/§7), delegating the
// per-bead value to build. errCtx prefixes the hard-error messages so each
// caller keeps its own diagnostic phrasing. It is a free function, not a method,
// because Go methods cannot carry type parameters.
func warpScan[V any](a *App, epic, errCtx string, build func(warpChild) V) (map[string]V, error) {
	res, err := run.BD(a.Runner, "list", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("%s: bd list could not run: %v", errCtx, err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("%s: bd list failed: %s", errCtx, strings.TrimSpace(res.Stderr))
	}
	var arr []warpChild
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("%s: parse bd list json: %v", errCtx, err)
	}
	m := map[string]V{}
	for _, it := range arr {
		for _, l := range it.Labels {
			if strings.HasPrefix(l, plan.RefLabelPrefix) {
				m[strings.TrimPrefix(l, plan.RefLabelPrefix)] = build(it)
			}
		}
	}
	return m, nil
}

// warpRefMap reads an epic's children and rebuilds the ref->bead map from their
// weft-ref:<ref> labels (spec §3/§7) in a single bd list call.
func (a *App) warpRefMap(epic string) (map[string]plan.ExistingBead, error) {
	return warpScan(a, epic, "warp-ref-map", func(it warpChild) plan.ExistingBead {
		return plan.ExistingBead{ID: it.ID, Status: it.Status}
	})
}

// warpReadback re-reads an epic's children after import and returns a map keyed
// by ref (from weft-ref:<ref> labels) suitable for VerifyReplan. It is a
// separate reader from warpRefMap because it reads fresh post-import state and
// needs different fields (title, priority, labels, description) than the
// pre-import warpRefMap snapshot.
func (a *App) warpReadback(epic string) (map[string]plan.ReadbackBead, error) {
	return warpScan(a, epic, "post-import read-back", func(it warpChild) plan.ReadbackBead {
		return plan.ReadbackBead{
			ID:          it.ID,
			Title:       it.Title,
			Priority:    it.Priority,
			Labels:      it.Labels,
			Description: it.Description,
		}
	})
}

// replanText renders the re-plan summary, flagging the §8-deferred items.
func replanText(epic string, rp plan.Replan, dry bool) string {
	prefix := "re-planned"
	if dry {
		prefix = "DRY RUN (upsert)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s epic %s — %d updated, %d created, %d removed\n",
		prefix, epic, len(rp.Updated), len(rp.Created), len(rp.Removed))
	if len(rp.DeferredEdges) > 0 {
		parts := make([]string, len(rp.DeferredEdges))
		for i, e := range rp.DeferredEdges {
			parts[i] = fmt.Sprintf("%s->%s", e.From, e.To)
		}
		verb := "wired post-import"
		if dry {
			verb = "will wire post-import"
		}
		fmt.Fprintf(&b, "  %d edge(s) touch a new pick — %s: %s\n",
			len(rp.DeferredEdges), verb, strings.Join(parts, ", "))
	}
	if len(rp.Removed) > 0 {
		verb := "closed (removed by replan)"
		if dry {
			verb = "will close (removed by replan)"
		}
		fmt.Fprintf(&b, "  %d removed ref(s) %s: %s\n", len(rp.Removed), verb, strings.Join(rp.Removed, ", "))
	}
	if len(rp.RemovedBlocked) > 0 {
		// Only reachable on dry-run: a live replan hard-fails before this text
		// renders when RemovedBlocked is non-empty (I2).
		fmt.Fprintf(&b, "  %d removed ref(s) BLOCKED — only open picks are removable; any non-open status (in_progress, closed, blocked, hooked, deferred, pinned, or unknown) blocks a live replan (I2): %s\n",
			len(rp.RemovedBlocked), strings.Join(rp.RemovedBlocked, ", "))
	}
	if dry {
		b.WriteString("  (no mutation — re-run without --dry-run to upsert)")
	}
	return b.String()
}

func (a *App) newPlanCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <file>",
		Short: "Validate warp-plan.json; validity is data (exit 0, no mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			issues := plan.Validate(wp)
			data := map[string]any{"valid": len(issues) == 0, "issues": issues}
			text := fmt.Sprintf("valid: %d pick(s), no issues", len(wp.Picks))
			if len(wp.Phases) > 0 {
				text = fmt.Sprintf("valid: %d phase(s), no issues", len(wp.Phases))
			}
			if len(issues) > 0 {
				var b strings.Builder
				fmt.Fprintf(&b, "INVALID: %d issue(s)", len(issues))
				for _, is := range issues {
					if is.Ref != "" {
						fmt.Fprintf(&b, "\n  - [%s] %s", is.Ref, is.Message)
					} else {
						fmt.Fprintf(&b, "\n  - %s", is.Message)
					}
				}
				text = b.String()
			}
			return Emit(cmd, "plan.check", data, text)
		},
	}
}
