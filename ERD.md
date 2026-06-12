# LOINC Browser ERD

This document summarizes the relationship model in the LOINC release artifacts and how the browser currently stores those relationships.

## LOINC Release Relationship Model

```mermaid
erDiagram
    LOINC_TERM {
        string LOINC_NUM PK
        string LONG_COMMON_NAME
        string COMPONENT
        string PROPERTY
        string TIME_ASPCT
        string SYSTEM
        string SCALE_TYP
        string METHOD_TYP
        string CLASS
        string STATUS
    }

    MAP_TO {
        string LOINC FK
        string MAP_TO FK
        string COMMENT
    }

    SOURCE_ORGANIZATION {
        string ID PK
        string COPYRIGHT_ID
        string NAME
        string COPYRIGHT
        string TERMS_OF_USE
        string URL
    }

    PART {
        string PartNumber PK
        string PartTypeName
        string PartName
        string PartDisplayName
        string Status
    }

    LOINC_PART_LINK {
        string LoincNumber FK
        string PartNumber FK
        string PartTypeName
        string LinkTypeName
        string Property
    }

    ANSWER_LIST {
        string AnswerListId PK
        string AnswerStringId
        string DisplayText
        string Score
    }

    LOINC_ANSWER_LIST_LINK {
        string LoincNumber FK
        string AnswerListId FK
        string AnswerListLinkType
        string ApplicableContext
    }

    PANEL_LINK {
        string ParentLoinc FK
        string Loinc FK
        string SEQUENCE
        string EntryType
        string DisplayNameForForm
        string AnswerListIdOverride
    }

    GROUP {
        string GroupId PK
        string ParentGroupId
        string Group
        string Archetype
        string Status
    }

    GROUP_LOINC_TERM {
        string GroupId FK
        string LoincNumber FK
        string Category
        string Archetype
    }

    COMPONENT_HIERARCHY {
        string CODE
        string IMMEDIATE_PARENT
        string PATH_TO_ROOT
        string CODE_TEXT
    }

    LOINC_TERM ||--o{ MAP_TO : "deprecated term maps to"
    LOINC_TERM ||--o{ MAP_TO : "replacement target"

    LOINC_TERM ||--o{ LOINC_PART_LINK : "has parts"
    PART ||--o{ LOINC_PART_LINK : "defines axis or semantic part"

    LOINC_TERM ||--o{ LOINC_ANSWER_LIST_LINK : "uses answer list"
    ANSWER_LIST ||--o{ LOINC_ANSWER_LIST_LINK : "contains answers"

    LOINC_TERM ||--o{ PANEL_LINK : "parent panel"
    LOINC_TERM ||--o{ PANEL_LINK : "child item"

    LOINC_TERM ||--o{ GROUP_LOINC_TERM : "member of group"
    GROUP ||--o{ GROUP_LOINC_TERM : "groups terms"

    LOINC_TERM ||--o{ COMPONENT_HIERARCHY : "leaf term in hierarchy"
    PART ||--o{ COMPONENT_HIERARCHY : "branch node"
```

## Conceptual Map

```mermaid
flowchart LR
    Term["LOINC Term"] --> Axes["Six major axes<br/>Component / Property / Time / System / Scale / Method"]
    Term --> Status["Status + MapTo<br/>active, discouraged, deprecated"]
    Term --> Parts["LOINC Parts<br/>primary, supplementary, radiology, document ontology"]
    Term --> Answers["Answer Lists<br/>allowed coded responses"]
    Term --> Panels["Panels and Forms<br/>parent-child form structure"]
    Term --> Groups["LOINC Groups<br/>value-set style rollups"]
    Term --> Hierarchy["Component Hierarchy by System<br/>tree browsing"]
    Term --> Copyright["Source Organization<br/>copyright/source metadata"]
```

## Current Browser Storage Model

The browser currently keeps the main term table normalized enough for search and facets, and stores accessory relationships in a generic table keyed by `kind`.

```mermaid
erDiagram
    loinc_terms {
        string loinc_num PK
        string component
        string property
        string time_aspect
        string system
        string scale
        string method
        string class
        string status
        string raw_json
    }

    loinc_terms_fts {
        string loinc_num
        string long_common_name
        string short_name
        string component
        string related_names
        string consumer_name
        string definition
        string display_name
        string system
        string property
        string scale
        string method
        string class
    }

    map_to {
        string loinc_num
        string map_to
        string comment
    }

    source_organizations {
        string id PK
        string copyright_id
        string name
        string copyright
        string terms_of_use
        string url
        string raw_json
    }

    term_accessories {
        int id PK
        string kind
        string loinc_num
        string code
        string title
        string subtitle
        string raw_json
    }

    import_meta {
        string key PK
        string value
    }

    loinc_terms ||--o{ loinc_terms_fts : "indexed by"
    loinc_terms ||--o{ map_to : "replacement mappings"
    loinc_terms ||--o{ term_accessories : "parts answers panels groups hierarchy"
```

## `term_accessories.kind`

Current values:

- `part-primary`
- `part-supplementary`
- `answer-list`
- `panel-membership`
- `panel-child`
- `group`
- `hierarchy`

This generic table lets the app ingest and browse relationship artifacts immediately. If a workflow becomes central, it can later be promoted into a dedicated normalized table, such as:

- `loinc_part_links`
- `loinc_answer_list_links`
- `loinc_panel_links`
- `loinc_group_members`
- `loinc_hierarchy_nodes`

