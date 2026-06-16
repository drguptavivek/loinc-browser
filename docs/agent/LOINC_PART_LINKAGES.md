# LOINC Part Linkages

## Part Linkages

LOINC enriched part linkages describe how LOINC terms connect to LOINC Parts in `LoincPartLink_Primary.csv` and `LoincPartLink_Supplementary.csv`. The enhanced model began in LOINC 2.66 and separates core FSN attributes, detailed semantic-model subparts, supplementary syntax fragments, semantic enhancements, metadata, and specialized radiology/document ontology attributes.

Implementation implication: do not treat all part links as equivalent. `LinkTypeName`, `PartTypeName`, `Property`, and `PartCodeSystem` determine whether a row is a primary defining axis, a decomposed name fragment, an external semantic linkage candidate, metadata, or a domain-specific model attribute.

Source: [LOINC Knowledge Base, "Enriched Linkages between LOINC terms and LOINC Parts"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Linkage Files

Since LOINC 2.68, the original part-link file has been split into:

- `LoincPartLink_Primary.csv`: Primary properties plus DocumentOntology and Radiology properties.
- `LoincPartLink_Supplementary.csv`: DetailedModel, SyntaxEnhancement, SemanticEnhancement, Metadata, Search, and other supplementary properties.

The split exists because the full link set is large and because implementers need a clear boundary around high-value attributes that should usually be implemented for each term.

Implementation implication: import both files when advanced search or semantic navigation is needed, but keep primary links available even in compact deployments. The supplementary file can be too large for spreadsheet tooling, so process it with streaming CSV import code rather than Excel-style workflows.

Source: [LOINC Knowledge Base, "Enriched Linkages between LOINC terms and LOINC Parts"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Link Types

The enriched model organizes links into these broad `LinkTypeName` groups:

- `Primary`: the five or six key Parts that make up the Fully-Specified Name.
- `DetailedModel`: compositional subparts from the official LOINC semantic model.
- `SyntaxEnhancement`: useful name fragments that are not official semantic-model parts.
- `SemanticEnhancement`: extra semantic subtyping or links toward external ontologies and vocabularies.
- `Metadata`: coded accessory attributes that are not part of the structured term name.
- `Radiology`: LOINC/RSNA Radiology Playbook attributes.
- `DocumentOntology`: LOINC Document Ontology attributes.
- `Search`: synonyms, fragments, and other Parts used to broaden search.

Not every LOINC term has every link type.

Source: [LOINC Knowledge Base, "Enriched Linkages between LOINC terms and LOINC Parts"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Primary Linkages

`Primary` links represent the exact six-axis field values from the LOINC table: Component, Property, Time, System, Scale, and Method when Method is specified. Every term should have at least five and at most six Primary links.

Implementation implication: Primary links are the safest part links for term-detail display, axis filtering, FHIR CodeSystem properties, and verifying that imported part links match the canonical LOINC table fields. The set of Primary part names should exactly match the term's major-axis field strings.

Source: [LOINC Knowledge Base, "Primary"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Detailed Model Linkages

`DetailedModel` links decompose the official LOINC semantic model into subparts such as analyte, challenge, adjustment, count, time-core, time-modifier, system-core, super-system, scale, and method. These links are still part of the official semantic model, but are more granular than the Primary axis fields.

Implementation implication: DetailedModel links help agents and UI tools explain why two terms differ below the full-axis level. They should not be mixed with SyntaxEnhancement links when reconstructing official term meaning.

Source: [LOINC Knowledge Base, "DetailedModel"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Syntax Enhancement

`SyntaxEnhancement` links identify useful fragments of the LOINC name that are not official semantic-model parts, such as analyte core, analyte suffix, numerator, divisor, and divisor suffix. These fragments can support search, translation, display explanation, and descriptive text linkage.

Implementation implication: use SyntaxEnhancement for search and explanatory UI, not as canonical term definition. It can expose important fragments hidden inside complex Components, but it does not replace Primary or DetailedModel links.

Source: [LOINC Knowledge Base, "Syntax Enhancement"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Semantic Enhancement

`SemanticEnhancement` links add subtype information or linkable semantic concepts that are not part of the official LOINC name model. Current examples focus on gene-type parts and illustrate future links to authoritative vocabularies such as HGNC, ClinVar, CHEBI, RxNorm, UBERON, RadLex, NCBI, HL7, or UNII.

Implementation implication: treat these links as advanced semantic enrichment. They can support ontology-aware search, grouping, and external terminology navigation, but the page describes this as work in progress and not a replacement for LOINC Parts.

Source: [LOINC Knowledge Base, "Semantic Enhancement"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Metadata Linkages

`Metadata` links are coded accessory attributes that are not part of the structured LOINC term name. Examples include class and category-style properties for grouping, indexing, and display.

Implementation implication: keep metadata linkages separate from defining semantic links. They are useful for browsing and filtering but should not be treated as part of the Fully-Specified Name.

Source: [LOINC Knowledge Base, "Metadata"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Radiology Linkages

`Radiology` links use specialized properties from the LOINC/RSNA Radiology Playbook, such as imaging focus, laterality, region imaged, modality type/subtype, pharmaceutical route/substance, reason for exam, subject, timing, and view attributes.

Implementation implication: radiology terms with Radiology links do not also use DetailedModel links because the radiology model supersedes the generic detailed model for those terms.

Source: [LOINC Knowledge Base, "Radiology"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Document Ontology Linkages

`DocumentOntology` links use specialized document attributes such as document kind, role, setting, subject matter domain, and type of service.

Implementation implication: document terms with DocumentOntology links do not also use DetailedModel links because the Document Ontology model supersedes the generic detailed model for those terms.

Source: [LOINC Knowledge Base, "DocumentOntology"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).

## Part Linkage Import Guidance

Recommended import behavior for this repository:

- Import `LoincPartLink_Primary.csv` and `LoincPartLink_Supplementary.csv` into one normalized `loinc_part_links` table with `link_type_name`, `part_type_name`, `property`, and `part_code_system` preserved.
- Use `Primary` links for compact term-detail axis metadata and high-confidence filters.
- Use `DetailedModel` links for explanation, comparison, and precise mapping support.
- Use `SyntaxEnhancement` and `Search` links for expanded search behavior, but label them clearly.
- Use `SemanticEnhancement` links for advanced ontology-aware workflows and keep external `PartCodeSystem` values visible.
- Keep `Radiology` and `DocumentOntology` link types distinct because they supersede the generic DetailedModel for their respective term families.
- Do not infer official term meaning from supplementary links unless the `LinkTypeName` semantics support that use.

Source: [LOINC Knowledge Base, "Enriched Linkages between LOINC terms and LOINC Parts"](https://loinc.org/kb/enriched-linkages-between-loinc-terms-and-loinc-parts/).
