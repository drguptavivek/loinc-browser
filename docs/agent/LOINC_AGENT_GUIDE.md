# LOINC Agent Guide

## Start

For unfamiliar tasks, first call `loinc_explain_concepts` with a focused topic such as `purpose`, `scope`, `term_identity`, `major_parts`, `five_parts`, `abbreviations`, `names`, `display_names`, `search_strategy`, `status`, `usage`, `answer_lists`, `panels`, `special_cases`, `microbiology`, `antimicrobial_susceptibility`, `database_structure`, `part_linkages`, `primary_linkages`, `semantic_enhancement`, `map_to_table`, `source_organization`, `official_api_search`, `official_api_query_syntax`, or `copyright`. Avoid loading full resources unless the user asks for documentation.

## Search For An Observation Term

Use `loinc_search_terms` with `usageType=observation`, `rankMode=observation`, and a compact limit. Prefer active terms by default. Follow up with `loinc_get_term_fit` and then `loinc_get_term` only for the selected candidate.

## Search The Official LOINC API

Use the local `/api/v1/official/search` proxy when the user explicitly asks for official LOINC API search or wants official search syntax behavior. Use `loinc_explain_concepts` with `topic=official_api_search` or `topic=official_api_query_syntax` first if query syntax is unclear.

The official proxy supports `loincs`, `answerlists`, `parts`, and `groups` scopes. Credentials are sent in a JSON `POST` body or loaded from encrypted local saved credentials; never put credentials in URLs or logs.

## Search For An Order Term

Use `loinc_search_terms` with `usageType=order`, `rankMode=order`, and `rankedOnly=true` when the user wants common orderables. Check status and fit metadata before recommending a term.

## Validate A Selected Term

Call `loinc_get_term_fit` for a compact suitability view. If the term has answer lists, panel items, hierarchy membership, or copyright metadata, call the specific follow-up tool instead of requesting full term detail.

## Check Scope

Use `loinc_explain_concepts` with `topic=scope` when deciding whether LOINC is appropriate for a requested concept. LOINC covers laboratory specimen observations, clinical observations about patients, discrete measurements, questions, documents, panels, batteries, and other collections.

## Choose A LOINC Name

Use `loinc_explain_concepts` with `topic=names` when deciding between Fully-Specified Name, Long Common Name, and Short Name. Prefer the Fully-Specified Name for mapping and disambiguation, the Long Common Name for official human-readable display or exchange metadata, and the Short Name only for constrained displays.

When a message, resource, implementation guide, or data dictionary includes a LOINC code, include an official LOINC name with it. Keep local codes paired with local names instead of placing local names in official LOINC display fields.

## Interpret Abbreviations

Use `loinc_explain_concepts` with `topic=abbreviations` when a term part abbreviation is unclear. Do not infer meaning from punctuation or abbreviation patterns alone; use term and part metadata when mapping or explaining a candidate.

## Handle Special Cases

Use `loinc_explain_concepts` with `topic=special_cases` when ordinary six-part matching is not enough. Use narrower topics such as `binary_vs_multiple_answer`, `blood_bank`, `flow_cytometry`, `microbiology`, `antimicrobial_susceptibility`, `molecular_genetics`, `allergy`, or `urinalysis_strips` when the domain is known.

## Inspect Database Structure

Use `loinc_explain_concepts` with `topic=database_structure` when reasoning about import fields, schema design, or release-file provenance. Use narrower topics such as `loinc_table`, `change_tracking`, `status_fields`, `name_fields`, `map_to_table`, `source_organization`, or `import_guidance` for implementation work.

## Inspect Part Linkages

Use `loinc_explain_concepts` with `topic=part_linkages` when reasoning about `LoincPartLink_Primary.csv`, `LoincPartLink_Supplementary.csv`, `LinkTypeName`, `PartTypeName`, `Property`, or `PartCodeSystem`. Use narrower topics such as `primary_linkages`, `detailed_model_linkages`, `syntax_enhancement`, `semantic_enhancement`, `radiology_linkages`, `document_ontology_linkages`, or `part_linkage_import_guidance` when implementing import, search, or semantic UI behavior.

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
