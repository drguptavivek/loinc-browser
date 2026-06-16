# LOINC Database Structure

## Database Structure

The LOINC database structure page is a technical field reference for the core LOINC table and related support tables. For this app, treat `LoincTable/Loinc.csv` as the canonical term source and use accessory files to normalize relationships, source/copyright metadata, panels, answer lists, parts, groups, and hierarchy.

The core engineering rule is that `LOINC_NUM` is the stable term identifier, while the six major axes, names, status fields, rank fields, and relationship/accessory files supply interpretation and workflow context.

Source: [LOINC Users' Guide, "A - LOINC Database Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## LOINC Table

`LoincTable/Loinc.csv` is the canonical row-level source for LOINC terms. Important field groups:

- Identifier: `LOINC_NUM`.
- Major axes: `COMPONENT`, `PROPERTY`, `TIME_ASPCT`, `SYSTEM`, `SCALE_TYP`, `METHOD_TYP`.
- Classification: `CLASS`, `CLASSTYPE`.
- Version/change tracking: `VersionFirstReleased`, `VersionLastChanged`, `CHNG_TYPE`, `CHANGE_REASON_PUBLIC`.
- Status: `STATUS`, `STATUS_REASON`, `STATUS_TEXT`.
- Names/display: `SHORTNAME`, `LONG_COMMON_NAME`, `DisplayName`, `CONSUMER_NAME`.
- Use/workflow: `ORDER_OBS`, `PanelType`, `AskAtOrderEntry`, `AssociatedObservations`, `ValidHL7AttachmentRequest`, `HL7_ATTACHMENT_STRUCTURE`.
- Units/examples: `EXAMPLE_UNITS`, `EXAMPLE_UCUM_UNITS`, `UNITSREQUIRED`, `EXMPL_ANSWERS`.
- Survey/form fields: `SURVEY_QUEST_TEXT`, `SURVEY_QUEST_SRC`.
- Search/context fields: `RELATEDNAMES2`, `DefinitionDescription`, `FORMULA`.
- Copyright/source linkage: `EXTERNAL_COPYRIGHT_NOTICE`, `EXTERNAL_COPYRIGHT_LINK`.
- Rank signals: `COMMON_TEST_RANK`, `COMMON_ORDER_RANK`.

Implementation note: use typed destination columns rather than raw JSON blobs when importing these fields. Keep the original LOINC field names visible in API documentation and import code so future release drift is easy to audit.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Identifier Fields

`LOINC_NUM` is a string in the LOINC code format and should be the primary stable key for term-level APIs, UI routes, search result selection, and relationship lookups. Do not derive meaning from the numeric string itself; term meaning comes from the row fields and relationships.

Accessory tables may use other identifiers, such as answer list IDs, answer string IDs, part numbers, group IDs, hierarchy node IDs, and source copyright IDs. Keep those identifiers distinct from `LOINC_NUM`; do not collapse them into one generic ID namespace.

Source: [LOINC Users' Guide, "A - LOINC Database Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Axis Fields

The six major LOINC axes are stored as separate fields in the term table:

- `COMPONENT`: first major axis, analyte/component observed.
- `PROPERTY`: second major axis, property observed.
- `TIME_ASPCT`: third major axis, timing.
- `SYSTEM`: fourth major axis, specimen or system.
- `SCALE_TYP`: fifth major axis, scale of measurement.
- `METHOD_TYP`: sixth major axis, method of measurement.

Implementation note: index these fields for filtering and expose them in term detail. Search ranking can use them as structured facets; mapping workflows should compare them explicitly rather than relying only on display names.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Change Tracking

`VersionFirstReleased` and `VersionLastChanged` identify when a term first appeared and when the record last changed. `CHNG_TYPE` indicates the category of the last change. Important change categories include:

- `ADD`: concept added.
- `DEL`: concept deprecated.
- `PANEL`: panel child or conditionality changed.
- `NAM`: `COMPONENT` changed.
- `MAJ`: one of the other major axes changed.
- `MIN`: non-major metadata changed.
- `UND`: concept moved from deprecated to active, trial, or discouraged.

Implementation note: use change fields for release-diff displays, migration warnings, and cache invalidation. A major-axis change has stronger mapping implications than a display-name or metadata-only change.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Status Fields

`STATUS` is operationally important for mapping and recommendations:

- `ACTIVE`: normally usable.
- `TRIAL`: experimental; use with caution because concept attributes may change.
- `DISCOURAGED`: not recommended for current use; existing mappings may remain valid in context.
- `DEPRECATED`: retained for history and should not be used for new mappings.

`STATUS_REASON` can explain non-active status with values such as ambiguous, duplicate, or erroneous. `STATUS_TEXT` gives narrative context. `CHANGE_REASON_PUBLIC` provides additional change history.

Implementation note: search should prefer active terms and make non-active status visible in compact results. Deprecated and discouraged terms should be linked to replacement guidance when `MapTo` data exists.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Name Fields

LOINC publishes several name/display fields:

- `SHORTNAME`: compact algorithmic short name.
- `LONG_COMMON_NAME`: readable common name.
- `DisplayName`: clinician-friendly algorithmic display name.
- `CONSUMER_NAME`: experimental consumer-friendly name.
- `RELATEDNAMES2`: synonyms for parts of the fully specified name.

Implementation note: use `LONG_COMMON_NAME` or `DisplayName` for primary UI display, depending on the product context. Keep the six-axis fields available for technical disambiguation and mapping. Do not use `SHORTNAME` as a unique key.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Order Observation Fields

`ORDER_OBS` indicates intended use as order-only, observation-only, both, or subset. The source page notes that this field reflects LOINC's best approximation of intended use and is not a binding normative resolution.

`PanelType` describes panel-like terms as panel, convenience group, or organizer. `AskAtOrderEntry` and `AssociatedObservations` are semicolon-delimited LOINC code lists for optional order-entry questions or associated observations.

Implementation note: form builders and order workflows should filter by `ORDER_OBS` but still inspect fit metadata, panel items, and local workflow requirements before recommending a code.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Units And Examples

`EXAMPLE_UNITS` contains representative units seen from submitters or users, but these are examples rather than required or recommended units. `EXAMPLE_UCUM_UNITS` gives example UCUM expressions. `UNITSREQUIRED` indicates whether units are required in certain HIPAA attachment OBX use cases. `EXMPL_ANSWERS` provides example valid answers for some terms.

Implementation note: do not use example units as validation-only authority. For UI assistance, show them as examples and prefer UCUM handling where possible.

Source: [LOINC Users' Guide, "LOINC Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Map To Table

The `MapTo` table supports replacement guidance for deprecated or discouraged terms:

- `LOINC`: deprecated or discouraged source term.
- `MAP_TO`: recommended replacement term.
- `COMMENT`: rationale or guidance for the replacement.

Implementation note: model this as term-to-term replacement edges. A deprecated term may have replacement guidance, but replacement still needs human or workflow validation when meaning changed or multiple replacements exist.

Source: [LOINC Users' Guide, "MapTo Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Source Organization

`SourceOrganization` provides source and copyright metadata:

- `ID`: internal numeric source identifier.
- `COPYRIGHT_ID`: foreign-key value referenced from the LOINC table.
- `NAME`: source organization name.
- `COPYRIGHT`: copyright notice.
- `TERMS_OF_USE`: source-specific use terms.
- `URL`: reference URL.

Implementation note: link `EXTERNAL_COPYRIGHT_LINK` / `COPYRIGHT_ID` values to normalized source records. Surface this metadata before export, redistribution, or UI display of third-party copyrighted content.

Source: [LOINC Users' Guide, "SourceOrganization Table Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).

## Import Guidance

For this repository, import the LOINC release into normalized SQLite tables while keeping generated data out of git. The app treats `LoincTable/Loinc.csv` as canonical for term APIs and imports accessory files for relationships and metadata.

The generated SQLite database also preserves every original source CSV as its own generated `raw_csv_*` table. These raw tables are named deterministically from the release-relative CSV path, include `_row_number`, and use the original CSV header names as SQLite columns where possible. They are for lossless local audit and future field promotion; normalized `/api/v1` routes should continue to read the typed tables.

Recommended import behavior:

- Preserve LOINC field names in import code and documentation.
- Store major axes, status, names, rank, and workflow fields in typed columns.
- Normalize relationship-like data from accessory files rather than embedding it as unstructured blobs.
- Preserve source CSVs in generated raw tables so no release columns are lost when the normalized schema intentionally stores only typed, API-ready fields.
- Keep replacement mappings, source organizations, panel items, answer lists, parts, groups, and hierarchy as separate tables.
- Treat source/copyright fields as first-class metadata for export and display decisions.
- Make release version fields queryable so users can audit drift across LOINC releases.

Source: [LOINC Users' Guide, "A - LOINC Database Structure"](https://loinc.org/kb/users-guide/loinc-database-structure/).
