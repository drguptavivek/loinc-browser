---
name: loinc-mcp
description: Use when an agent needs to connect to this repository's local LOINC MCP server, search LOINC terms, inspect fit metadata, panels, answer lists, hierarchy, parts, groups, understand LOINC concepts, or use the local FHIR terminology (lookup, validate-code, subsumes, ValueSet expand/validate/search, ConceptMap translate, Questionnaire, Lucene search).
---

# LOINC MCP Agent Skill

## Connect

Prefer stdio when the agent can launch local commands:

```json
{
  "mcpServers": {
    "loinc": {
      "command": "/path/to/loinc-browser",
      "args": ["mcp"]
    }
  }
}
```

Use HTTP when the default all-in-one server is running locally:

```text
http://localhost:9005/mcp
```

The usual command is:

```bash
loinc-browser
```

With Claude Code specifically:

```bash
claude mcp add --transport http loinc http://localhost:9005/mcp
```

The server negotiates any protocol version the client offers (`2026-07-28` down to `2024-11-05`);
Claude Code currently negotiates `2025-11-25` via the classic `initialize` handshake. See
`docs/MCP.md` for protocol-version details.

## Note on stdio scope

The stdio transport (`loinc-browser mcp`) opens one fixed database for the process lifetime.
`loinc_lucene_search` works there too, reading the local Bleve index at `--search-index-path`
(default `./data/loinc-search.bleve`); if that index has not been built yet
(`POST /api/v1/local-search/rebuild` on the HTTP server), the tool reports a clear error instead of
guessing.

## First Calls

If unfamiliar with LOINC, call `loinc_explain_concepts` with a focused topic such as `search_strategy`, `status`, `usage`, `answer_lists`, `panels`, or `hierarchy`.

## Tool Choices

- Use `loinc_search_terms` for compact term candidates.
- Use `loinc_get_term_fit` before recommending a selected term.
- Use `loinc_get_term` only after selecting a specific LOINC number.
- Use `loinc_get_term_relationships` to discover linked answer lists, panels, parts, groups, and hierarchy.
- Use `loinc_search_panels` and `loinc_get_panel_items` for forms and panels.
- Use `loinc_search_answer_lists` and `loinc_get_answer_list_answers` for coded answer choices.
- Use `loinc_browse_hierarchy` and `loinc_get_hierarchy_terms` for hierarchy workflows.
- Use `loinc_search_parts` and `loinc_search_groups` to broaden or compare related terms.
- Use `loinc_lookup_code` for a compact FHIR-grade view of any code kind (term/LP/LL/LA/LG),
  including axis properties and MAP_TO/parent/child relations, when `loinc_get_term` alone is not
  enough (e.g. to see the six-axis fully-specified-name parts, or a deprecated term's MAP_TO
  target before deciding whether to translate).
- Use `loinc_validate_code` to confirm a code is valid and active (and that a candidate display
  string matches) before handing it to a downstream system.
- Use `loinc_subsumes` to compare two codes/parts via the Component Hierarchy by System — e.g. to
  check whether a candidate code is broader/narrower than one already in use.
- Use `loinc_expand_value_set` / `loinc_search_value_sets` / `loinc_validate_value_set_membership`
  for ValueSet work: named catalogue sets (`loinc-all`, `deprecated-loinc-terms`, ...), LL answer
  lists, LG groups, and implicit LP part-hierarchy sets.
- Use `loinc_get_questionnaire` to get a panel's authored item tree in FHIR Questionnaire shape
  (linkId/code/text/type/required/answerOptions) when building a structured form.
- Use `loinc_lucene_search` for Lucene-style fielded/boolean/wildcard/range queries the compact
  `loinc_search_*` tools don't expose.

### Mapping workflow: search -> lookup -> validate -> translate deprecated

1. `loinc_search_terms` to find candidates for a concept.
2. `loinc_lookup_code` (or `loinc_get_term_fit`) on the best candidate to confirm axes, class, and
   status before recommending it.
3. `loinc_validate_code` to confirm the code/display pair is valid and active before using it in an
   integration.
4. If the term's status is `DEPRECATED` (or `loinc_lookup_code` shows a `MAP_TO` relation), call
   `loinc_translate` with `{"code": "<deprecated code>", "id": "loinc-map-to"}` to find its
   replacement, then repeat step 3 on the replacement code. Use `loinc_list_concept_maps` first if
   unsure which served ConceptMap to target.

## Context Rules

Keep calls compact. Use small limits, pagination, and default detail. Request `detail=full` only when the user needs full metadata.

Always preserve stable identifiers: LOINC numbers, answer list IDs, part numbers, group IDs, and hierarchy node IDs.

## License Rules

Do not copy licensed LOINC release files, extracted release directories, generated SQLite databases, WAL/SHM files, or bulk release dumps into source control or external prompts. Prefer narrow MCP calls.

Inactive, deprecated, or discouraged terms should not be recommended unless the user explicitly asks for them or the task is legacy mapping.
