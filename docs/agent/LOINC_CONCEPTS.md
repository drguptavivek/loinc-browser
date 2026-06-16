# LOINC Concepts For Agents

This is the lightweight index for the agent-facing LOINC knowledge base. Use `loinc_explain_concepts` with a focused topic instead of loading whole files when possible.

## Purpose

LOINC provides universal identifiers and names for laboratory and clinical observations so results can be exchanged, pooled, and interpreted across heterogeneous systems. Use a LOINC code when the task is to identify an observation, orderable test, survey question, clinical document, panel, or related health measurement concept. LOINC is not meant to carry every operational detail about the observation; instrument, collection-site detail, priority, verifier, sample size, and place of testing usually belong in other message fields.

Source: [LOINC Users' Guide, "1 - Introduction"](https://loinc.org/kb/users-guide/introduction/).

## Scope

LOINC covers things that can be tested, measured, or observed about a patient. Its two broad divisions are Laboratory and Clinical.

Laboratory content covers observations about specimens, including chemistry, hematology, serology, microbiology, virology, parasitology, toxicology, cell counts, antibiotic susceptibilities, and related laboratory domains.

Clinical content covers observations about a patient that do not require removing a specimen, including vital signs, hemodynamics, intake/output, EKG, imaging observations, procedure-related observations, radiology studies, clinical documents, selected survey instruments, and patient assessment measures.

LOINC has terms for both discrete observations and collections. Discrete observations include single lab tests, survey questions, patient measurements, and report elements. Collections include panels, batteries, question sets, groups of clinical measurements, documents, and reports.

Source: [LOINC, "Scope of LOINC"](https://loinc.org/get-started/scope-of-loinc/).

## Search Strategy

Start broad with compact search results, then narrow by status, usage type, rank mode, class, system, scale, method, hierarchy node, part, or group. Validate selected terms with fit metadata and inspect answer lists or panel items when building forms.

For mapping, prioritize the Fully-Specified Name and its major parts. Compare local meaning against Component, Property, Time, System, Scale, and Method before relying on display-name similarity.

Sources: [LOINC Term Basics](https://loinc.org/get-started/loinc-term-basics/) and [LOINC Users' Guide, "2 - Major Parts of a LOINC term"](https://loinc.org/kb/users-guide/major-parts-of-a-loinc-term/).

## Topic Map

- Term structure: see `LOINC_TERM_STRUCTURE.md` for `term_identity`, `major_parts`, `five_parts`, `abbreviations`, `component`, `property`, `time`, `system`, `scale`, `method`, `status`, `usage`, `rank`, `panels`, `answer_lists`, `hierarchy`, `parts`, and `groups`.
- Names and display: see `LOINC_NAMES_AND_DISPLAY.md` for `names` and `display_names`.
- Special cases: see `LOINC_SPECIAL_CASES.md` for `special_cases`, `binary_vs_multiple_answer`, `blood_bank`, `flow_cytometry`, `microbiology`, `antimicrobial_susceptibility`, `molecular_genetics`, `allergy`, and `urinalysis_strips`.
- Database structure: see `LOINC_DATABASE_STRUCTURE.md` for `database_structure`, `loinc_table`, `identifier_fields`, `axis_fields`, `change_tracking`, `status_fields`, `name_fields`, `order_observation_fields`, `units_and_examples`, `map_to_table`, `source_organization`, and `import_guidance`.
- Part linkages: see `LOINC_PART_LINKAGES.md` for `part_linkages`, `linkage_files`, `link_types`, `primary_linkages`, `detailed_model_linkages`, `syntax_enhancement`, `semantic_enhancement`, `metadata_linkages`, `radiology_linkages`, `document_ontology_linkages`, and `part_linkage_import_guidance`.
- Official API: see `LOINC_OFFICIAL_API.md` for `official_api_search`, `official_api_credentials`, `official_api_query_syntax`, and `official_api_use_guidance`.
- License: see `LOINC_LICENSE_NOTE.md` for `copyright`.

All source-derived sections should include a direct Markdown link to the LOINC or related source page.
