# Clinical Relationship Lanes Design

Date: 2026-06-14

## Goal

Make the LOINC relationship graph and term drawer more useful to clinicians by organizing relationships around clinical tasks rather than exposing a generic network first.

Clinical Relationship Lanes are the dominant default view. The existing broader relationship graph should be retained as an alternate exploration view for users who want the older network-oriented behavior.

The priority order is:

1. Order set and panel building.
2. Questionnaire and form mapping.
3. Result interpretation.
4. General terminology exploration.

The design must answer two common questions quickly:

- Which common panels, orders, questionnaires, scales, or forms contain this selected item?
- If this selected term is a panel, order, questionnaire, scale, or form, what items does it contain?

## Clinical Lanes

The drawer and graph use the same relationship lanes so clinicians do not need to learn two different models.

### Clinical Role

Show a compact summary of the selected term:

- LOINC number and preferred name.
- Status.
- Order, observation, both, or neither.
- Panel/order/questionnaire/survey item indicators inferred from class, method, order/observation flag, and available relationships.
- Answer-list indicator when the term has answer-list links.
- Core axes: class, system, scale, method, property.

### Contained In

Shows parent containers that include the selected item.

Source:

- `panelMemberships`.

Contents:

- Parent panels.
- Parent orders or order sets.
- Parent questionnaires, scales, survey instruments, and forms.

Sorting:

1. Ranked parents first, low numeric rank first.
2. Parent type priority: questionnaire/scale/form, then panel/order, then other.
3. Selected item sequence within the parent.
4. Parent name and LOINC number.

Each row should show:

- Parent LOINC code.
- Parent title.
- Type badge.
- Rank if present.
- Sequence of the selected item in that parent if present.

### Contains

Shows child observations, child questions, or child form items when the selected term is itself a container.

Source:

- `panelItems`.

Sorting:

1. Panel or questionnaire sequence.
2. Ranked/common children when sequence is missing or duplicated.
3. Child name and LOINC number.

Rules:

- Deduplicate repeated child LOINC codes in the default view.
- Preserve duplicate source row count.
- Preserve item IDs and source details for expandable details.

Each row should show:

- Sequence.
- Child LOINC code.
- Child name.
- Status.
- Answer-list indicator when present.
- Duplicate source-row count when greater than one.

### Hierarchy Path

Shows hierarchical parents as a separate lane after explicit clinical relationships.

Source:

- `hierarchy`.

Drawer behavior:

- Show the nearest available path-like hierarchy parents.
- Keep this below `Contained in` and `Contains`.

Graph behavior:

- Show the nearest two or three hierarchy parents by default.
- Make additional hierarchy parents available through lane expansion.

### Nearby Context

Shows context that helps interpret the selected term without overwhelming the primary clinical lanes.

Priority:

1. Sibling items from the most relevant parent container.
2. Other siblings from other parent containers.
3. Broader shared concept neighborhoods.

Shared concepts remain collapsed by default under a broader related-terms area.

## Graph Behavior

The graph starts as a clinical map, not a dense free graph.

Default mode:

- Clinical Lanes is the default graph mode.
- The older network-style graph remains available as an alternate view, labeled for exploration rather than clinical workflow.
- Switching views should not discard the selected term or loaded relationship data.

Default layout:

- Parent containers above the selected term.
- Selected term in the center.
- Child items below.
- Hierarchy and nearby context to the side.

Expansion:

- `More` expands each lane deliberately rather than adding arbitrary nodes.
- Parent and child lanes should have independent limits where practical.
- Shared concepts stay secondary and should not crowd out explicit parent or child relationships.

Interactions:

- Clicking a parent panel/questionnaire focuses that container and shows its child items in sequence.
- Clicking a child item opens that item while preserving a path back to the parent context.
- Clicking a hierarchy parent switches to hierarchy browsing while keeping the selected term visible in the drawer.
- The drawer remains the readable source of truth; the graph is the visual summary.

Alternate exploration view:

- Preserve the current relationship graph behavior for broader concept discovery.
- Keep shared concepts and non-lane relationships visible there.
- Use this view for terminology exploration, debugging relationships, and cases where the clinical lane model is too narrow.
- Do not make this the default view for clinicians.

## Drawer Behavior

The drawer should be optimized for reading and clinical decision support.

The drawer follows the Clinical Lanes model by default. If the alternate graph view is selected, the drawer may still show the same lane sections, with broader shared-concept sections available below them.

Default order:

1. Clinical role.
2. Contained in.
3. Contains.
4. Hierarchy path.
5. Nearby context.
6. Broader shared concepts.
7. Raw fields.

The drawer should show explicit relationship sections even when the graph is not open. Loading relationship details may still be lazy, but once loaded the sections should be clearly separated.

## Data And API Notes

The current API already exposes the core sources:

- `/api/v1/terms/{loincNum}/relationships`
- `panelMemberships`
- `panelItems`
- `hierarchy`
- `parts`
- `answerLists`
- `groups`
- `sharedConcepts`

Potential backend additions:

- Parent class, method, order/observation flag, and rank fields on `panelMemberships`.
- Child class, status, answer-list indicator, and rank fields on `panelItems`.
- A computed relationship lane/type field to avoid duplicating classification logic in the frontend.
- A sibling endpoint or relationship payload section for siblings from selected parent containers.

## Testing

Use representative examples:

- CBC panel `58410-2`: must show contained observations in sequence.
- PHQ or survey item examples: must show parent questionnaire or scale under `Contained in`.
- Survey/questionnaire container examples: must show child questionnaire items under `Contains`.
- Generic observation examples: must show hierarchy path and broader context only after explicit parent relationships.

Verification:

- `go test ./...`
- `npm --prefix web run check`
- `npm --prefix web run build`
- Browser verification at `http://localhost:9005`.

## Out Of Scope

- Editing LOINC data.
- Online authoring of relationship documentation.
- Replacing the hierarchy browser.
- Making shared concepts the primary graph driver.
