# LOINC Agent Guide

## Start

For unfamiliar tasks, first call `loinc_explain_concepts` with a focused topic such as `search_strategy`, `status`, `usage`, `answer_lists`, or `panels`. Avoid loading full resources unless the user asks for documentation.

## Search For An Observation Term

Use `loinc_search_terms` with `usageType=observation`, `rankMode=observation`, and a compact limit. Prefer active terms by default. Follow up with `loinc_get_term_fit` and then `loinc_get_term` only for the selected candidate.

## Search For An Order Term

Use `loinc_search_terms` with `usageType=order`, `rankMode=order`, and `rankedOnly=true` when the user wants common orderables. Check status and fit metadata before recommending a term.

## Validate A Selected Term

Call `loinc_get_term_fit` for a compact suitability view. If the term has answer lists, panel items, hierarchy membership, or copyright metadata, call the specific follow-up tool instead of requesting full term detail.

## Inspect Answer Lists

Use `loinc_get_term_relationships` or `loinc_search_answer_lists` to identify answer list IDs. Then call `loinc_get_answer_list_answers` with pagination. For form work, preserve answer order.

## Inspect Panels And Forms

Use `loinc_search_panels` to find panels or forms. Then call `loinc_get_panel_items` for authored child items, required flags, form labels, datatypes, and answer list overrides.

## Browse Hierarchy

Use `loinc_browse_hierarchy` with no node ID for roots, then follow child `nodeId` values. Use `loinc_get_hierarchy_terms` to retrieve terms under a specific occurrence subtree.

## Compare Candidates

Keep comparison calls compact. Request only summaries until a short candidate list is available. Then fetch fit metadata and focused details for the few candidates that matter.

## Context Discipline

Use small limits and pagination. Prefer `detail=summary` or default detail. Use `detail=full` only when the user needs full metadata or when a narrow summary is insufficient.
