---
name: stackyrd-docs-wiki
description: >
  Update docs_wiki/ when code changes, user asks, or README.md falls out of
  sync with docs_wiki/. NOT for auto-generated Swagger docs.
---

# stackyrd Documentation Wiki Guide

Keep `docs_wiki/` accurate and in sync with the codebase. `docs_wiki/README.md` is the index.

## Conventions

- `# Title` + one-sentence summary at top of every doc.
- Fenced code blocks with language tags (` ```go `, ` ```yaml `, etc.). Go snippets use module path `stackyrd`.
- Mermaid `flowchart TD/LR` for architecture/flows, `stateDiagram-v2` for state machines.
- Tables for reference data (endpoints, config fields, component lists). Clear headers, consistent alignment.
- No emojis, no conversational language. One topic per file.

## Workflow

1. **Read** the existing doc (if any) and the **actual source code** — verify interfaces, config keys, import paths.
2. **Write/update** the doc file per conventions above.
3. **Update** `docs_wiki/README.md` — add/remove/modify table entry.
4. **Check** other doc files for cross-references that need updating.
5. **Update** `AGENTS.md` if a file was added/removed and it's listed in the directory tree.

Cross-ref: `stackyrd-dev` skill for extension patterns before writing about them.
