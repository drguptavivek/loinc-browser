# LOINC Concepts For Agents

## Term

A LOINC term identifies one clinical observation, orderable test, panel, survey question, or related health measurement concept. Use the LOINC number as the stable identifier for follow-up calls. Prefer narrow lookups by LOINC number after search.

## Axes

LOINC terms are described by six main axes: component, property, time aspect, system, scale, and method. These axes explain what is measured, the kind of property, timing, specimen or system, result scale, and method when a method is specified.

## Status

Status matters for agent recommendations. Active terms are normally preferred. Deprecated, discouraged, and inactive terms require caution and should be selected only when the task explicitly needs them or when mapping legacy data.

## Usage

`ORDER_OBS` describes whether a term is suitable as an order, an observation, or both. For form-builder work, match the intended workflow: observation terms for captured results, order terms for ordering workflows, and both only when appropriate.

## Rank

Common test and common order ranks are usage signals. Lower positive ranks are more common. Use `rankMode=observation` for result-capture workflows and `rankMode=order` for ordering workflows. `rankedOnly=true` narrows results to commonly used terms.

## Panels

Panels and forms are LOINC terms that contain authored child items. Use panel tools when building questionnaires, lab panels, or forms. Inspect panel items before assuming the parent term alone is enough.

## Answer Lists

Answer lists define allowed answer choices for coded questions. A term may link to one or more answer lists, and a panel item may override the answer list. Inspect answer choices before using a term in a structured form.

## Hierarchy

Hierarchy browsing uses occurrence `nodeId` values. Do not use hierarchy concept codes as tree state because a concept can appear in multiple branches. Use node IDs for children, breadcrumbs, and subtree term calls.

## Parts

Parts are reusable pieces of LOINC term meaning, such as components, systems, methods, and other axes. Part searches help agents find related terms when direct term search is too broad.

## Groups

Groups collect related LOINC terms for clinical or domain-oriented browsing. Group membership can help compare candidates, but final selection should still use term detail and fit metadata.

## Copyright

Some terms or related metadata may have external copyright/source constraints. Use copyright/source endpoints when exporting, displaying, or reusing detailed metadata outside local search.

## Search Strategy

Start broad with compact search results, then narrow by status, usage type, rank mode, class, system, scale, method, hierarchy node, part, or group. Validate selected terms with fit metadata and inspect answer lists or panel items when building forms.
