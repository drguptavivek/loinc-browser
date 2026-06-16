# Local Lucene Search

This document describes the local Lucene-style search index planned and exposed by the LOINC Browser. The local index is a generated search artifact built from the normalized SQLite database. SQLite remains the canonical source for term, part, answer-list, and group detail records.

## Storage and lifecycle

- Default index path: `./data/loinc-search.bleve/`
- Environment override: `LOINC_SEARCH_INDEX_PATH`
- Index engine: Bleve v2, an embedded Go search engine with Lucene-style query strings
- Lifecycle: manual rebuild first, through `POST /api/v1/local-search/rebuild`
- Status: `GET /api/v1/local-search/status`

The index is generated local data and must not be committed. If the local SQLite database lacks future expanded search-source tables, status should report partial coverage or `requires_reingest` for fields that cannot be populated from the current normalized schema.

## Search scopes

| Scope | Document id | Canonical source | Returned local object |
| --- | --- | --- | --- |
| `loincs` | `loinc:{LOINC_NUM}` | `loinc_terms` plus relationship tables | `SearchResult` |
| `parts` | `part:{part_number}` | `parts` plus derived/link metadata | `Part` |
| `answerlists` | `answerlist:{answer_list_id}` | `answer_lists`, `answer_list_answers`, links | `AnswerList` |
| `groups` | `group:{group_id}` | `loinc_groups`, `parent_groups`, links | `LOINCGroup` |

Every document includes:

- `scope`: exact scope filter
- `key`: canonical local id
- `_all`: combined analyzed text for broad searches
- official-style field names, such as `Component`, `System`, `AnswerDisplayText`, or `Part`

## Field types

| Type | Purpose |
| --- | --- |
| `text` | analyzed full-text fields such as names, descriptions, synonyms, answer display text |
| `keyword` | exact or case-normalized fields such as codes, IDs, status, class, type, OIDs |
| `bool` | true/false fields such as `AnswerList:true` or `Methodless:true` |
| `number` | ranks, counts, sequence numbers, molecular weight |
| `date` | date-like values such as `CreatedOn` in `YYYYMMDD` format |
| `derived` | computed fields such as `ComponentWordCount`, `Methodless`, `SuperSystem` |
| `relationship` | fields populated from link tables, such as `AnswerListId` or `MapToLOINC` |

## Basic query syntax

| Syntax | Example | Behavior |
| --- | --- | --- |
| quoted phrase | `influenza "virus A"` | phrase is searched as one unit |
| implicit AND | `morphine cutoff` | both terms are expected |
| `AND` | `morphine AND cutoff` | both terms are expected |
| `OR` | `influenza OR parainfluenza` | either term may match |
| `NOT` | `influenza NOT equine` | excludes following term |
| plus | `morphine +cutoff` | requires following term |
| minus | `morphine -equine` | excludes following term |
| wildcard `*` | `artemi*` | multi-character wildcard |
| wildcard `?` | `80619-?` | single-character wildcard |
| field search | `Component:opiates System:hair` | searches a specific indexed field |

## Advanced query syntax

The local advanced search parser supports the following Lucene-style syntax for indexed fields:

| Syntax | Example | Behavior |
| --- | --- |
| parentheses | `(influenza OR rhinovirus) -haemophilus` | groups boolean clauses |
| field grouping | `Component:(opiates OR confirm)` | applies one field to a grouped expression |
| fuzzy search | `haemofhilus~` | permits fuzzy term matching; bare `~` uses up to two edits, and `~1` or `~2` can be explicit |
| proximity search | `"function panel"~1` | accepts phrase-slop syntax; local execution currently treats it as a phrase match |
| inclusive range | `Rank:[80 TO 100]` | includes both endpoints for indexed numeric or keyword range fields |
| exclusive range | `Rank:{80 TO 100}` | excludes both endpoints for indexed numeric or keyword range fields |
| escaped syntax characters | `Class:DRUG\\/TOX` | searches values that contain reserved syntax characters |

## LOINC term fields

Basic official LOINC fields:

| Field | Type | Source or derivation |
| --- | --- | --- |
| `LOINC` | keyword | `loinc_terms.loinc_num` |
| `Component` | text | `component` |
| `Property` | keyword/text | `property` |
| `Timing` | keyword/text | `time_aspect` |
| `System` | keyword/text | `system` |
| `Scale` | keyword/text | `scale` |
| `Method` | text | `method` |
| `Class` | keyword | `class` |

Advanced LOINC fields:

| Field | Type | Source or derivation |
| --- | --- | --- |
| `AllowMethodSpecific` | bool/derived | future expanded source |
| `AnswerList` | bool/relationship | answer-list links exist |
| `AnswerListId` | keyword/relationship | `loinc_answer_list_links.answer_list_id` |
| `AnswerListName` | text/relationship | `loinc_answer_list_links.answer_list_name` |
| `AnswerListType` | keyword/relationship | future expanded source |
| `AskAtOrderEntry` | keyword/bool/relationship | future expanded source |
| `AssociatedObservations` | keyword/bool/relationship | future expanded source |
| `AttachmentUnitsRequired` | keyword | `units_required` |
| `Categorization` | bool/relationship | future category source |
| `ClassHierarchy` | text/relationship | hierarchy/category source |
| `ComponentHierarchy` | text/relationship | hierarchy/category source |
| `MethodHierarchy` | text/relationship | hierarchy/category source |
| `MultiAxialHierarchy` | text/relationship | hierarchy/category source |
| `SystemHierarchy` | text/relationship | hierarchy/category source |
| `CommonOrder` | bool/number | `common_order_rank > 0` |
| `Ranked` | bool/number | `common_test_rank > 0` |
| `CommonLabResult` | bool/number | alias of `Ranked` |
| `ComponentWordCount` | number/derived | word count of `component` |
| `CoreComponent` | text/derived | initially component-derived; expanded source later |
| `Description` | text | `definition` |
| `DisplayName` | text | `display_name` |
| `ExUCUMunits` | text | future expanded source |
| `ExUnits` | text | future expanded source |
| `Formula` | text | future expanded source |
| `HL7AttachmentStructure` | keyword/text | future expanded source |
| `HL7FieldSubId` | keyword/text | future expanded source |
| `LabTest` | bool/derived | `class_type == 1` when available |
| `LForms` | bool | future expanded source |
| `LongName` | text | `long_common_name` |
| `MapToLOINC` | keyword/relationship | `loinc_map_to.target_loinc_num` |
| `MassProperty` | bool/derived | property classification |
| `Methodless` | bool/derived | method is empty |
| `NonroutineChallenge` | bool | future expanded source |
| `OrderObs` | keyword | `order_obs` |
| `OtherCopyright` | text/bool/relationship | source organization/copyright metadata |
| `PanelType` | keyword | future expanded source |
| `Pharma` | bool | future expanded source |
| `Punctuation` | keyword/derived | punctuation in core name axes |
| `Rank` | number | `common_test_rank` |
| `RelatedCodes` | keyword/text | future related-code source |
| `ShortName` | text | `short_name` |
| `Status` | keyword | `status` |
| `StatusReason` | keyword | future expanded source |
| `StatusText` | text | future expanded source |
| `SubstanceProperty` | bool/derived | property classification |
| `SuperSystem` | text/derived | text after `^` in system |
| `SurveyQuestionSource` | text | future expanded source |
| `SurveyQuestionText` | text | future expanded source |
| `TimeModifier` | text/derived | text after `^` in timing |
| `Type` | number/keyword | class type when available |
| `TypeName` | keyword/derived | class type name when available |
| `UniversalLabOrders` | bool/relationship | future value-set source |
| `ValidHL7AttachmentRequest` | keyword/bool | future expanded source |
| `VersionLastChanged` | keyword | future expanded source |

Class-based filtering examples:

```text
Class:CHEM
Class:DRUG/TOX
TypeName:Lab
TypeName:Clinical
Status:ACTIVE
```

## Part fields

| Field | Type | Source or derivation |
| --- | --- | --- |
| `Partnumber` | keyword | `parts.part_number` |
| `Part` | text | `parts.part_name` |
| `Name` | text | alias of `Part` |
| `Abbreviation` | text | future expanded source |
| `Article` | bool | future reference source |
| `Book` | bool | future reference source |
| `Citation` | bool | future reference source |
| `ClassList` | keyword/text | derived from linked LOINC classes |
| `CreatedOn` | date/keyword | future expanded source |
| `Description` | text/bool | future reference source |
| `DisplayName` | text | `parts.part_display_name` |
| `Image` | bool | future reference source |
| `MolecularWeight` | number/bool | future expanded source |
| `OriginalForm` | bool | future reference source |
| `PackageInsert` | bool | future reference source |
| `Synonyms` | text | future expanded source |
| `TechnicalBrief` | bool | future reference source |
| `Type` | keyword | `parts.part_type_name` |
| `WebContent` | bool | future reference source |

## Answer-list fields

| Field | Type | Source or derivation |
| --- | --- | --- |
| `AnswerList` | keyword | `answer_lists.answer_list_id` |
| `Name` | text | `answer_lists.answer_list_name` |
| `Description` | text | future expanded source |
| `AnswerCode` | keyword/text | local and external answer codes |
| `AnswerCodeSystem` | keyword/text | local and external answer code systems |
| `LOINCAnswerListOID` | keyword | `answer_lists.answer_list_oid` |
| `AnswerCount` | number | count of answer rows |
| `AnswerDisplayText` | text | `answer_list_answers.display_text` |
| `AnswerScore` | keyword/number | `answer_list_answers.score` |
| `AnswerSequenceNum` | number/keyword | `answer_list_answers.sequence_number` |
| `AnswerString` | keyword | answer string id / LA code |
| `AnswerStringDescription` | text | answer description |
| `CodeSystem` | keyword/text | alias of answer code system |
| `ExternalAnswerListOID` | keyword | future expanded source |
| `ExternalListURL` | keyword/text | `ext_defined_answer_list_link` |
| `ExternallyDefined` | bool | `ext_defined_yn` |
| `LoincCount` | number | count of linked LOINC terms |
| `SourceName` | text/keyword | future expanded source |

## Group fields

| Field | Type | Source or derivation |
| --- | --- | --- |
| `Group` | keyword | `loinc_groups.group_id` |
| `GroupId` | keyword | alias of `Group` |
| `Name` | text | `loinc_groups.group_name` |
| `Archetype` | keyword/text | `loinc_groups.archetype` |
| `ParentGroup` | keyword/text | `parent_groups.parent_group` |
| `Status` | keyword | `loinc_groups.status` |
| `VersionFirstReleased` | keyword | `loinc_groups.version_first_released` |
| `LoincCount` | number | count of linked group terms |

## Planned API

```http
GET /api/v1/local-search/status
POST /api/v1/local-search/rebuild
POST /api/v1/local-search/query
```

Query request:

```json
{
  "scope": "loincs",
  "query": "Component:morphine AND cutoff",
  "limit": 25,
  "offset": 0
}
```

Query response:

```json
{
  "scope": "loincs",
  "query": "Component:morphine AND cutoff",
  "results": [],
  "total": 0,
  "limit": 25,
  "offset": 0,
  "warnings": [],
  "indexStatus": "ready"
}
```
