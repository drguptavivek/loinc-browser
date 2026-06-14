# LOINC Browser ERD

This document summarizes the relationship model in the LOINC release artifacts and how the browser stores and queries those relationships. The `/api/v1` routes read the normalized tables directly; relationship data is not copied into compatibility tables or raw JSON columns.

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

The browser now imports relationship data into normalized tables with foreign keys. `loinc_terms` remains the canonical term table and `loinc_terms_fts` remains the search index. Relationship data is no longer written into generic compatibility tables.

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

    parts {
        string part_number PK
        string part_type_name
        string part_name
        string part_display_name
        string status
    }

    loinc_part_links {
        string loinc_num FK
        string part_number FK
        string link_set
        string link_type_name
        string property
    }

    answer_lists {
        string answer_list_id PK
        string answer_list_name
        string answer_list_oid
    }

    answer_list_answers {
        string answer_list_id FK
        string answer_string_id
        int sequence_number
        string display_text
        string score
    }

    loinc_answer_list_links {
        string loinc_num FK
        string answer_list_id FK
        string answer_list_link_type
        string applicable_context
    }

    loinc_map_to {
        string loinc_num
        string target_loinc_num
        string comment
    }

    source_organizations {
        string id PK
        string copyright_id
        string name
        string copyright
        string terms_of_use
        string url
    }

    panel_items {
        string parent_loinc_num FK
        string child_loinc_num FK
        int sequence
        string entry_type
        string answer_list_id_override FK
    }

    parent_groups {
        string parent_group_id PK
        string parent_group
        string status
    }

    loinc_groups {
        string group_id PK
        string parent_group_id FK
        string group_name
        string archetype
        string status
    }

    group_loinc_terms {
        string group_id FK
        string loinc_num FK
        string category
        string archetype
    }

    hierarchy_concepts {
        string code PK
        string label
        string node_kind
        string loinc_num FK
        string part_number FK
    }

    hierarchy_occurrences {
        int node_id PK
        string code
        int parent_node_id FK
        string path_key
        int occurrence_ordinal
        int subtree_term_count
    }

    hierarchy_closure {
        int ancestor_node_id FK
        int descendant_node_id FK
        int depth
    }

    hierarchy_subtree_terms {
        int node_id FK
        string loinc_num FK
        int descendant_node_id FK
        int distance
    }

    import_meta {
        string key PK
        string value
    }

    loinc_terms ||--o{ loinc_terms_fts : "indexed by"
    loinc_terms ||--o{ loinc_map_to : "deprecated term"
    loinc_terms ||--o{ loinc_map_to : "replacement target"
    loinc_terms ||--o{ loinc_part_links : "has parts"
    parts ||--o{ loinc_part_links : "used by terms"
    loinc_terms ||--o{ loinc_answer_list_links : "uses answer list"
    answer_lists ||--o{ loinc_answer_list_links : "linked from terms"
    answer_lists ||--o{ answer_list_answers : "contains answers"
    loinc_terms ||--o{ panel_items : "parent panel"
    loinc_terms ||--o{ panel_items : "child observation"
    parent_groups ||--o{ loinc_groups : "contains groups"
    loinc_groups ||--o{ group_loinc_terms : "has members"
    loinc_terms ||--o{ group_loinc_terms : "member term"
    hierarchy_concepts ||--o{ hierarchy_occurrences : "appears at paths"
    hierarchy_occurrences ||--o{ hierarchy_occurrences : "parent occurrence"
    hierarchy_occurrences ||--o{ hierarchy_closure : "ancestor"
    hierarchy_occurrences ||--o{ hierarchy_subtree_terms : "subtree terms"
    loinc_terms ||--o{ hierarchy_subtree_terms : "descendant term"
```

Hierarchy codes are concept identifiers, not unique tree positions. The same code can appear in more than one branch, so `hierarchy_occurrences` stores path occurrences using `node_id`, `path_key`, and `occurrence_ordinal`; `hierarchy_concepts` stores the unique code identity. API hierarchy browsing and branch-scoped term queries use `node_id`.
