---
name: loinc-mcp
description: Use when an agent needs to connect to this repository's local LOINC MCP server, search LOINC terms, inspect fit metadata, panels, answer lists, hierarchy, parts, groups, or understand LOINC concepts.
---

# LOINC MCP Agent Skill

## Connect

Prefer stdio when the agent can launch local commands:

```json
{
  "mcpServers": {
    "loinc": {
      "command": "/path/to/loinc-browser",
      "args": ["mcp", "--db", "./data/loinc-normalized.sqlite", "--docs-dir", "./docs/agent"]
    }
  }
}
```

Use HTTP when `loinc-browser serve --mcp` is already running locally:

```text
http://localhost:8080/mcp
```

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

## Context Rules

Keep calls compact. Use small limits, pagination, and default detail. Request `detail=full` only when the user needs full metadata.

Always preserve stable identifiers: LOINC numbers, answer list IDs, part numbers, group IDs, and hierarchy node IDs.

## License Rules

Do not copy licensed LOINC release files, extracted release directories, generated SQLite databases, WAL/SHM files, or bulk release dumps into source control or external prompts. Prefer narrow MCP calls.

Inactive, deprecated, or discouraged terms should not be recommended unless the user explicitly asks for them or the task is legacy mapping.
