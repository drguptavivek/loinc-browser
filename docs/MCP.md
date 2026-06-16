# LOINC MCP Guide

The LOINC Browser binary exposes a local Model Context Protocol (MCP) server for agents. It uses the same normalized SQLite database and Store behavior as `/api/v1`.

## Transports

### All-in-one HTTP

The default run mode starts the UI, `/api/v1`, Swagger UI, `/openapi.json`, and HTTP MCP together:

```bash
loinc-browser
```

The default MCP HTTP endpoint is:

```text
http://localhost:9005/mcp
```

`loinc-browser serve --addr ...` is equivalent. Use `--mcp-path` to change the HTTP MCP route, or `--no-mcp` to disable HTTP MCP.

### Stdio

Use stdio when an agent should launch a dedicated MCP process instead of connecting to the all-in-one HTTP server:

```bash
loinc-browser mcp --docs-dir ./docs/agent
```

## Editable Agent Docs

MCP resources read Markdown files from disk at request time. Edits made in a code editor are visible on the next resource/tool call.

`loinc://concepts` returns the lightweight concept index at `docs/agent/LOINC_CONCEPTS.md`. The `loinc_explain_concepts` tool searches topic sections across the structured agent KB files in `docs/agent/`, including term structure, names/display, special cases, database structure, part linkages, and license notes.

Default docs directory:

```text
./docs/agent
```

Override with:

```bash
--docs-dir ./docs/agent
LOINC_AGENT_DOCS_DIR=./docs/agent
```

## Tools

Context is capped by default. Use small limits and follow-up calls by stable ID.

| Tool | Purpose |
| --- | --- |
| `loinc_explain_concepts` | Return a compact explanation for one LOINC topic from editable Markdown. |
| `loinc_search_terms` | Search compact LOINC term candidates. |
| `loinc_get_term` | Get one selected LOINC term. |
| `loinc_get_term_fit` | Get compact form-builder suitability metadata. |
| `loinc_get_term_relationships` | Get grouped lightweight relationships. |
| `loinc_search_panels` | Search panels and forms. |
| `loinc_get_panel_items` | List panel/form items in authored sequence. |
| `loinc_search_answer_lists` | Search answer lists. |
| `loinc_get_answer_list_answers` | List answer choices in sequence. |
| `loinc_browse_hierarchy` | Browse hierarchy roots or children by occurrence `nodeId`. |
| `loinc_get_hierarchy_terms` | List terms under a hierarchy node. |
| `loinc_search_parts` | Search LOINC parts. |
| `loinc_search_groups` | Search LOINC groups. |

## Resources

| Resource | Source |
| --- | --- |
| `loinc://concepts` | `docs/agent/LOINC_CONCEPTS.md` |
| `loinc://agent-guide` | `docs/agent/LOINC_AGENT_GUIDE.md` |
| `loinc://license-note` | `docs/agent/LOINC_LICENSE_NOTE.md` |
| `loinc://api-guide` | `docs/API.md` |
| `loinc://openapi` | live OpenAPI JSON from the app |

## Context Discipline

- Search first with compact results.
- Keep `limit` small; MCP tools cap large limits.
- Use `loinc_get_term_fit` before recommending a selected code.
- Use focused follow-up tools for answer lists, panels, hierarchy, parts, and groups.
- Request full detail only when needed.
- Do not use MCP to produce bulk dumps of the LOINC release.

## License Discipline

LOINC release data is licensed. Do not commit or copy release zip files, extracted release directories, generated SQLite databases, WAL/SHM files, or bulk release dumps into source control or external prompts.
