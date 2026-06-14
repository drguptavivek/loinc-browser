# LOINC v1 API Guide

This document is the structured reference for the normalized LOINC API used by the local browser and by EMR form-builder clients. The machine-readable OpenAPI 3.1 document is served by the app at `/openapi.json`; this guide explains how to use the routes and how the route groups fit together.

The `/api/v1` routes are the primary API contract. They read normalized SQLite tables directly and avoid compatibility relationship tables, raw JSON fallback columns, and nested relationship payloads in term detail responses.

## API scope

Base URL in development:

```text
http://localhost:9005
```

Common alternate local URL:

```text
http://localhost:18080
```

Primary documentation endpoints:

| Route | Purpose |
| --- | --- |
| `/api/docs` | Swagger UI for interactive API browsing. |
| `/openapi.json` | OpenAPI 3.1 machine-readable contract. |
| `/api/v1/health` | v1 health check. |
| `/api/version` | Application version metadata for the UI, scripts, and release checks. |
| `/api/v1/version` | v1 version metadata endpoint. |
| `/docs/mcp` | Live Markdown MCP guide served from `docs/MCP.md`. |
| `/docs/concepts` | Live Markdown concept guide served from `docs/agent/LOINC_CONCEPTS.md`. |
| `/docs/agent-guide` | Live Markdown agent guide served from `docs/agent/LOINC_AGENT_GUIDE.md`. |

Operational endpoints outside `/api/v1`, such as `/api/import/upload`, may remain for local release loading and development. New API clients should use `/api/v1` for LOINC search, relationships, hierarchy browsing, and form-builder workflows.

## Design principles

- Search and list routes return small paginated summaries.
- Detail routes return one focused resource, not large nested relationship graphs.
- Relationship routes expose direct lightweight relationship records and HATEOAS links to deeper endpoints.
- Shared term-list routes use the same status, usage, rank, sort, and pagination semantics.
- Hierarchy browsing uses `nodeId`, not hierarchy concept code, so duplicate codes can be browsed safely.
- Relationship data is read from normalized tables such as `loinc_map_to`, `loinc_part_links`, `loinc_answer_list_links`, `panel_items`, `group_loinc_terms`, and hierarchy occurrence/subtree tables.

## HATEOAS

Responses include `_links` wherever the client needs a next action. Treat these links as navigational hints rather than hard-coding every follow-up route in UI code.

Typical term summary links:

```json
{
  "_links": {
    "self": "/api/v1/terms/1234-5",
    "fit": "/api/v1/terms/1234-5/fit",
    "relationships": "/api/v1/terms/1234-5/relationships"
  }
}
```

Typical page links:

```json
{
  "_links": {
    "self": "/api/v1/terms/search?q=glucose&limit=25&offset=0",
    "next": "/api/v1/terms/search?q=glucose&limit=25&offset=25",
    "prev": ""
  }
}
```

## Pagination

List responses use a page envelope:

```json
{
  "results": [],
  "total": 0,
  "limit": 25,
  "offset": 0,
  "hasMore": false,
  "_links": {
    "self": "",
    "next": "",
    "prev": ""
  }
}
```

Rules:

- `limit` defaults to `25` for most routes.
- Term-list `limit` is capped at `100`.
- `offset` defaults to `0`.
- Use `_links.next` for paging when present.
- Panel items and answer-list answers default to `limit=100` because they often represent authored form structure.

## Term list defaults

All shared term-list routes use the same defaults unless a route says otherwise:

| Parameter | Default | Meaning |
| --- | --- | --- |
| `q` | empty | Full-text search query or exact LOINC number. |
| `status` | not `INACTIVE` | Repeatable status filter. Use `status=INACTIVE` to search inactive terms, or `status=*` to include all statuses. |
| `usageType` | `any` | `any`, `observation`, or `order`. |
| `rankMode` | `observation` | Which rank field drives usage sorting and `rankedOnly`. |
| `sort` | `relevance` when `q` is present, otherwise `usage` | `relevance`, `usage`, or `alpha`. |
| `rankedOnly` | `false` | When true, require a positive rank in the selected `rankMode`. |
| `limit` | `25` | Maximum rows to return. Term-list maximum is `100`. |
| `offset` | `0` | Result offset. |

Additional term filters:

| Parameter | Meaning |
| --- | --- |
| `class` | LOINC class filter. |
| `system` | System axis filter. |
| `timeAspect` | Repeatable time aspect filter. |
| `scale` | Repeatable scale filter. |
| `method` | Repeatable method filter. |
| `property` | Property axis filter. |
| `orderObs` | Repeatable raw `ORDER_OBS` filter. |
| `hierarchyNodeId` | Restrict results to a hierarchy occurrence subtree. |

Usage filters:

- `usageType=observation` returns terms whose `ORDER_OBS` behaves as observation or both.
- `usageType=order` returns terms whose `ORDER_OBS` behaves as order or both.
- `usageType=any` does not restrict order/observation use.

Rank modes:

- `rankMode=observation` uses `commonTestRank`.
- `rankMode=order` uses `commonOrderRank`.
- Lower positive rank values are better for usage sorting.
- Unranked terms stay searchable unless `rankedOnly=true`.

## Core response shapes

### Term summary

Term-list routes return `TermSummary`-like results. These are intentionally compact:

```json
{
  "loincNum": "1234-5",
  "longCommonName": "Example term",
  "shortName": "Example",
  "displayName": "Example",
  "status": "ACTIVE",
  "orderObs": "Both",
  "usageTypes": ["observation", "order"],
  "commonTestRank": 1,
  "commonOrderRank": 10,
  "system": "Serum",
  "class": "CHEM",
  "scale": "Qn",
  "property": "MCnc",
  "_links": {}
}
```

### Term detail

`GET /api/v1/terms/{loincNum}` returns full term metadata plus links. It does not embed relationship lists. Use the relationship, answer-list, panel, hierarchy, part, and group endpoints for those use cases.

### Term fit

`GET /api/v1/terms/{loincNum}/fit` is a compact suitability view for form builders. It includes:

- status flags: deprecated, discouraged, inactive
- usage flags: order, observation, or both
- `commonTestRank` and `commonOrderRank`
- whether the term has answer lists, panel items, panel memberships, hierarchy membership, or external copyright metadata
- links back to the term and relevant relationship endpoints

## Complete v1 route map

### Health

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/health` | Check that the API server is responding. |

### Version

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/version` | Get app version, build commit, build date when present, and Go target platform. |
| GET | `/api/v1/version` | Same version metadata under the v1 API namespace. |

Example response:

```json
{
  "version": "0.90",
  "commit": "dev",
  "goos": "darwin",
  "goarch": "arm64"
}
```

### Terms

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/terms/search` | Search ranked LOINC terms for an EMR field. |
| GET | `/api/v1/terms/top` | Browse top ranked terms without a text query. |
| GET | `/api/v1/terms/{loincNum}` | Get one full term detail. |
| GET | `/api/v1/terms/{loincNum}/fit` | Get form-builder suitability metadata. |
| GET | `/api/v1/terms/{loincNum}/relationships` | Get grouped lightweight relationships. |
| GET | `/api/v1/terms/{loincNum}/answer-lists` | List answer lists linked to one term. |
| GET | `/api/v1/terms/{loincNum}/panel-memberships` | List panels that contain one term. |
| GET | `/api/v1/terms/{loincNum}/copyright` | Get copyright/source metadata state for one term. |

### Hierarchy

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/hierarchy/roots` | List root hierarchy nodes. |
| GET | `/api/v1/hierarchy/nodes/{nodeId}` | Get one hierarchy occurrence node. |
| GET | `/api/v1/hierarchy/nodes/{nodeId}/parents` | List parent nodes for breadcrumb navigation. |
| GET | `/api/v1/hierarchy/nodes/{nodeId}/children` | List immediate children. |
| GET | `/api/v1/hierarchy/nodes/{nodeId}/terms` | List terms under a hierarchy subtree. |

Hierarchy node responses include:

| Field | Meaning |
| --- | --- |
| `nodeId` | Stable occurrence identity used for browsing. |
| `code` | Hierarchy concept code. Not unique enough for browse state. |
| `label` | Display label. |
| `pathKey` | Path-preserving key from the imported hierarchy. |
| `parentNodeId` | Parent occurrence identity. |
| `termCount` | Terms under this occurrence. |
| `childCount` | Immediate children. |
| `_links` | Links to self, parents, children, and terms where available. |

### Panels and questionnaire forms

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/panels/search` | Search terms that act as panels or forms. |
| GET | `/api/v1/panels/{loincNum}` | Get panel term detail. |
| GET | `/api/v1/panels/{loincNum}/items` | List panel or questionnaire items in authored sequence. |

Panel item records include:

| Field | Meaning |
| --- | --- |
| `parentLoincNum` | Parent panel/form LOINC number. |
| `childLoincNum` | Child item LOINC number. |
| `sequence` | Authored item order. |
| `displayNameForForm` | Form display label from the panel artifact. |
| `observationRequired` | Whether the observation is required in the panel artifact. |
| `entryType` | Form entry type. |
| `dataTypeInForm` | Authored datatype in the form artifact. |
| `answerListIdOverride` | Optional answer-list override. May be empty. |
| `childTerm` | Compact child term summary. |
| `_links` | Links to child term and answer-list override when present. |

### Answer lists

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/answer-lists/search` | Search answer lists by id, name, or OID. |
| GET | `/api/v1/answer-lists/{answerListId}` | Get answer-list detail. |
| GET | `/api/v1/answer-lists/{answerListId}/answers` | List coded answer choices in sequence. |
| GET | `/api/v1/answer-lists/{answerListId}/terms` | List terms linked to an answer list. |

Answer-list answer rows are ordered by `sequenceNumber` and expose local code fields plus external code fields when available in the imported release.

### Parts

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/parts/search` | Search parts by part number, name, display name, or type. |
| GET | `/api/v1/parts/{partNumber}` | Get part detail. |
| GET | `/api/v1/parts/{partNumber}/terms` | List terms linked to a part. |

Part term lists support the shared term-list parameters. They also support:

| Parameter | Meaning |
| --- | --- |
| `linkSet` | Optional part link-set filter, for example primary or supplementary. |

### Groups

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/groups/search` | Search LOINC groups by id, name, archetype, or parent group. |
| GET | `/api/v1/groups/{groupId}` | Get group detail. |
| GET | `/api/v1/groups/{groupId}/terms` | List terms linked to a group. |

Group term lists support the shared status, usage, rank, sort, and pagination parameters.

### Source organizations and copyright

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/source-organizations` | List imported source organizations. |
| GET | `/api/v1/source-organizations/{id}` | Get source organization detail. |

Use `GET /api/v1/terms/{loincNum}/copyright` for term-level copyright state. If the imported typed schema cannot resolve term-level source organizations, the endpoint returns an explicit empty or unknown state instead of raw JSON.

### Accessories

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/v1/accessories` | Browse lightweight imported accessory records across kinds. |

Supported query parameters:

| Parameter | Meaning |
| --- | --- |
| `kind` | Optional accessory kind filter. |
| `q` | Text query over accessory titles, codes, subtitles, or linked terms. |
| `limit` | Maximum rows to return. Default `50`. |
| `offset` | Result offset. |

This route is useful for generic browse screens. For structured workflows, prefer the focused answer-list, panel, part, group, hierarchy, and relationship endpoints.

## EMR form-builder workflows

### 1. Find a term for a field

```bash
curl 'http://localhost:9005/api/v1/terms/search?q=glucose&usageType=observation&rankMode=observation&sort=relevance&limit=25'
```

Recommended UI behavior:

- Display `longCommonName`, `status`, `orderObs`, `usageTypes`, and rank values.
- Default to all non-inactive terms.
- Let users explicitly search inactive terms with `status=INACTIVE`, or include every status with `status=*`.
- Use `_links.self` for detail and `_links.relationships` for follow-up exploration.

### 2. Browse commonly used terms

```bash
curl 'http://localhost:9005/api/v1/terms/top?rankMode=observation&usageType=observation&rankedOnly=true&limit=25'
curl 'http://localhost:9005/api/v1/terms/top?rankMode=order&usageType=order&rankedOnly=true&limit=25'
```

Use observation rank for result-entry fields and order rank for order-entry fields.

### 3. Check whether a term is suitable

```bash
curl 'http://localhost:9005/api/v1/terms/1234-5/fit'
```

Use this before adding a term to a form field. It gives a compact status and relationship summary without loading all relationships.

### 4. Traverse the hierarchy

```bash
curl 'http://localhost:9005/api/v1/hierarchy/roots'
curl 'http://localhost:9005/api/v1/hierarchy/nodes/6729/children'
curl 'http://localhost:9005/api/v1/hierarchy/nodes/6729/terms?sort=usage&limit=25'
```

Use `nodeId` for UI state. Do not use hierarchy `code` as a tree key because the same code can occur under multiple paths.

### 5. Inspect relationships for a candidate term

```bash
curl 'http://localhost:9005/api/v1/terms/1234-5/relationships'
```

Relationship groups:

| Group | Meaning |
| --- | --- |
| `mapTo` | Outgoing replacement mappings for a term. |
| `mappedFrom` | Deprecated or alternate terms that map to this term. |
| `parts` | Linked semantic parts. |
| `answerLists` | Linked answer lists. |
| `panelMemberships` | Panels/forms containing the term. |
| `panelItems` | Items contained by the term when the term is a panel/form. |
| `groups` | Linked LOINC groups. |
| `hierarchy` | Hierarchy occurrences involving the term. |

### 6. Build a questionnaire or panel

```bash
curl 'http://localhost:9005/api/v1/panels/search?q=screening&limit=25'
curl 'http://localhost:9005/api/v1/panels/1234-5/items?limit=100'
```

Use `sequence` as the default sort for panel items. If an item has `answerListIdOverride`, follow its link or call the answer-list endpoint to load answer choices.

### 7. Load coded answer choices

```bash
curl 'http://localhost:9005/api/v1/terms/1234-5/answer-lists'
curl 'http://localhost:9005/api/v1/answer-lists/LL1234-5/answers?limit=100'
curl 'http://localhost:9005/api/v1/answer-lists/LL1234-5/terms?status=*&limit=25'
```

Use answer-list answer rows for coded choice controls. Use linked terms to find which LOINC terms use an answer list.

### 8. Explore parts and groups for alternatives

```bash
curl 'http://localhost:9005/api/v1/parts/search?q=glucose'
curl 'http://localhost:9005/api/v1/parts/LP12345-6/terms?linkSet=primary&sort=usage'
curl 'http://localhost:9005/api/v1/groups/search?q=chemistry'
curl 'http://localhost:9005/api/v1/groups/LG1234-5/terms?sort=usage'
```

Use parts to find terms sharing semantic axes. Use groups to find curated related term sets.

### 9. Check copyright and source metadata

```bash
curl 'http://localhost:9005/api/v1/terms/1234-5/copyright'
curl 'http://localhost:9005/api/v1/source-organizations'
curl 'http://localhost:9005/api/v1/source-organizations/1'
```

Use this before surfacing terms from externally copyrighted scales or source-controlled content.

## Efficient client pattern

For a form-builder UI, use this sequence:

1. Search or browse top terms with `/api/v1/terms/search` or `/api/v1/terms/top`.
2. Show compact `TermSummary` fields in the result list.
3. On selection, call `/api/v1/terms/{loincNum}` for full metadata.
4. Call `/api/v1/terms/{loincNum}/fit` for suitability flags.
5. Load relationships only when the user opens a relationship panel.
6. Use focused endpoints for answer lists, panels, hierarchy terms, part terms, and group terms.
7. Page every list route and follow `_links.next` when present.

This avoids N+1 relationship expansion and keeps default search fast.

## Errors and status codes

| Status | Meaning |
| --- | --- |
| `200` | Request succeeded. |
| `400` | Invalid input for routes that validate body or uploaded data. |
| `404` | Term, hierarchy node, answer list, part, group, or source organization was not found. |
| `503` | API server has no open store. |
| `500` | Unexpected database or server error. |

Error response shape:

```json
{
  "error": "message"
}
```

## Storage mapping for API users

| API area | Normalized tables used |
| --- | --- |
| Term search/detail | `loinc_terms`, `loinc_terms_fts` |
| Replacement mappings | `loinc_map_to` |
| Parts | `parts`, `loinc_part_links` |
| Answer lists | `answer_lists`, `answer_list_answers`, `loinc_answer_list_links` |
| Panels/forms | `panel_items` |
| Groups | `parent_groups`, `loinc_groups`, `group_loinc_terms` |
| Hierarchy | `hierarchy_concepts`, `hierarchy_occurrences`, `hierarchy_edges`, `hierarchy_closure`, `hierarchy_subtree_terms` |
| Source metadata | `source_organizations` |

See `ERD.md` for the database relationship diagram.
