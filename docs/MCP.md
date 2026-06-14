# LOINC MCP Guide

The LOINC Browser binary exposes a local Model Context Protocol (MCP) server for agents. It uses the same normalized SQLite database and Store behavior as `/api/v1`.

## Transports

### Stdio

Use stdio when an agent can launch a local command:

```bash
loinc-browser mcp --db ./data/loinc-normalized.sqlite --docs-dir ./docs/agent
```

### HTTP

Use HTTP when the browser/API server is already running:

```bash
loinc-browser serve --db ./data/loinc-normalized.sqlite --addr :8080 --mcp
```

The default MCP HTTP endpoint is:

```text
http://localhost:8080/mcp
```

Change it with `--mcp-path`.

## Editable Agent Docs

MCP resources read Markdown files from disk at request time. Edits made in a code editor are visible on the next resource/tool call.

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
