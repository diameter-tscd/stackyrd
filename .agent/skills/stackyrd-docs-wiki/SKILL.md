---
name: stackyrd-docs-wiki
description: Update docs_wiki/ when code changes, user asks, or README.md falls out of sync with docs_wiki/. Not for auto-generated Swagger docs.
---

# stackyrd Docs Wiki

Keep `docs_wiki/` accurate and in sync with code. `docs_wiki/README.md` is the index.

## Conventions

- `# Title` + one-sentence summary at top
- Fenced code blocks with language tags. Go snippets use module path `stackyrd`.
- Mermaid for architecture/flows
- Tables for reference data
- No emojis, no conversational language, one topic per file

## Workflow

1. Read the existing doc + actual source code (verify interfaces, config keys, import paths)
2. Write/update doc per conventions
3. Update `docs_wiki/README.md` index table entry
4. Check cross-references in other docs
5. Update `AGENTS.md` if a file was added/removed

Cross-ref: `.agent/skills/stackyrd-dev/SKILL.md` for extension patterns before writing about them.
