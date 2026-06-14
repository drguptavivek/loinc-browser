# Clinical Relationship Lanes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Clinical Relationship Lanes the default relationship view while preserving the existing Cytoscape relationship graph as an alternate exploration view.

**Architecture:** Implement a new Svelte component for the clinical lane map and lane lists, then wire it into the existing graph modal as the default tab. Reuse the existing `/api/v1/terms/{loincNum}/relationships` payload and existing relationship loading flow; keep the old `RelationshipGraph` component as the exploration tab. Tighten the drawer sections to match the same lane names and clinical role summary.

**Tech Stack:** Svelte 5, TypeScript, Tailwind CSS, existing Go API, existing Cytoscape exploration graph.

---

### Task 1: Add Clinical Lane Helpers And Component

**Files:**
- Create: `web/src/lib/components/ClinicalRelationshipLanes.svelte`

- [ ] **Step 1: Create lane component**

Create `ClinicalRelationshipLanes.svelte` with props for `term`, `graph`, `onOpenTerm`, and `onBrowseHierarchy`. The component groups relationship data into clinical lanes:

```svelte
<script lang="ts">
	import type { Term, TermAccessory, TermRelationshipGraph } from '$lib/api';
	import EmptyState from '$lib/components/EmptyState.svelte';

	type Props = {
		term: Term;
		graph?: TermRelationshipGraph | null;
		onOpenTerm?: (loincNum: string) => void;
		onBrowseHierarchy?: (nodeId: string, label: string) => void;
	};
</script>
```

- [ ] **Step 2: Add helper functions**

Implement helper functions for parent memberships, child items, hierarchy rows, deduplication, clinical type badges, and sequence labels. Keep the helpers local to the component for now because the transformation is presentation-specific.

- [ ] **Step 3: Render lanes**

Render a two-column layout:

- Left: visual lane map with `Contained in`, selected term, `Contains`, and side `Hierarchy path`.
- Right: readable lane sections with `Clinical role`, `Contained in`, `Contains`, `Hierarchy path`, and `Nearby context`.

- [ ] **Step 4: Verify Svelte check**

Run:

```bash
npm --prefix web run check
```

Expected: `svelte-check found 0 errors and 0 warnings`.

### Task 2: Make Clinical Lanes Default In Modal

**Files:**
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Import component**

Import `ClinicalRelationshipLanes` beside `RelationshipGraph`.

- [ ] **Step 2: Add graph view mode state**

Add:

```ts
let relationshipViewMode: 'clinical' | 'explore' = 'clinical';
```

Reset it to `clinical` when opening a new term.

- [ ] **Step 3: Add modal segmented control**

In the graph modal header, add buttons:

- `Clinical lanes`
- `Exploration graph`

Clinical lanes is selected by default. Exploration graph renders the existing `RelationshipGraph` component unchanged.

- [ ] **Step 4: Wire clinical lanes**

When `relationshipViewMode === 'clinical'`, render `ClinicalRelationshipLanes`. When `relationshipViewMode === 'explore'`, render `RelationshipGraph`.

- [ ] **Step 5: Verify old view remains available**

Browser verification must show the modal opens on Clinical lanes and the Exploration graph button switches to the old Cytoscape graph.

### Task 3: Align Drawer With Clinical Lanes

**Files:**
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Rename relationship drawer heading**

Change `Relationships and accessories` to `Clinical relationship lanes`.

- [ ] **Step 2: Add compact clinical role section**

Show status, order/observation value, class, system, scale, method, and whether there are parent containers, child items, or answer lists.

- [ ] **Step 3: Keep explicit lane order**

Use the existing `accessorySections` function but keep the order:

1. Parent panels / scales / orders.
2. Panel observations or scale / survey items.
3. Parts.
4. Answer lists.
5. Groups.
6. Hierarchy.

- [ ] **Step 4: Verify CBC panel**

Browser verification at `http://localhost:9005/?mode=facets&q=CBC+panel&sort=usage&term=58410-2` must show `Panel observations` with WBC, RBC, and sequence values.

### Task 4: Full Verification And Commit

**Files:**
- Test existing repo.

- [ ] **Step 1: Run Go tests**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Run Svelte check**

```bash
npm --prefix web run check
```

Expected: no errors or warnings.

- [ ] **Step 3: Run frontend build**

```bash
npm --prefix web run build
```

Expected: Vite build succeeds.

- [ ] **Step 4: Browser verify**

Restart `go run ./cmd/loinc-browser --port 9005` and verify:

- Clinical lanes are the default modal view.
- Exploration graph remains available.
- Drawer shows clinical lane wording.
- CBC panel contains child observations.

- [ ] **Step 5: Commit and push**

```bash
git add web/src/App.svelte web/src/lib/components/ClinicalRelationshipLanes.svelte docs/superpowers/plans/2026-06-14-clinical-relationship-lanes.md
git commit -m "feat: add clinical relationship lanes view"
git push origin main
```
