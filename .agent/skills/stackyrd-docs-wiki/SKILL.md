---
name: stackyrd-docs-wiki
description: >
  Maintain and update the stackyrd project's hand-written documentation in
  docs_wiki/. Use this skill whenever the user adds, modifies, or removes
  packages, middleware, services, infrastructure components, or plugins, and
  the corresponding doc files need updating. Also use when the user asks to
  update, write, fix, or reorganize documentation in docs_wiki/, or when the
  README.md table of contents falls out of sync with the files in docs_wiki/.
  If the user mentions any docs_wiki file by name, mentions "documentation"
  or "docs" in the context of the stackyrd project, asks to "keep docs in
  sync" with code changes, or says the TOC/README is outdated — trigger this
  skill. Do NOT use for auto-generated Swagger docs (docs/ directory).
---

# stackyrd Documentation Wiki Guide

**Before writing or updating any documentation, first load and apply the [ponytail](../ponytail/SKILL.md) skill.** The ponytail ladder governs docs too: YAGNI first (does this doc need to exist?), shortest doc that works, deletion over addition, no essays.

Apply the ladder before touching anything:
1. Does this doc need to exist? A one-line README entry often suffices.
2. Can a link to the source code replace a full doc?
3. Is the shortest possible doc (one section, no diagrams) enough?
4. Only then: the minimum documentation that covers the essential.

Keep the hand-written documentation in `docs_wiki/` accurate and in sync with the codebase. The `docs_wiki/` folder is the canonical source of project documentation, indexed by `docs_wiki/README.md`.

## When This Skill Should Run

1. **Code-driven updates** — when a service, middleware, infrastructure component, plugin, or utility package changes, the corresponding doc must be updated or created.
2. **Explicit doc requests** — when the user asks to update or write docs.
3. **TOC maintenance** — when files in docs_wiki/ are added, renamed, or removed.

## Key Files

| Path | Purpose |
|------|---------|
| `docs_wiki/README.md` | Table of contents — update whenever files in docs_wiki/ change |
| `docs_wiki/{TOPIC}.md` | Individual documentation files, one per topic area |
| `AGENTS.md` | Canonical policy: docs_wiki is the source of truth; update both doc and README |

## Documentation Conventions

All docs_wiki files follow these conventions.

### Structure
- Start every file with a `# Title` (H1) and a one-sentence summary line.
- Use `##` for major sections, `###` for subsections.
- Technical, professional tone. No emojis. No conversational language.
- Keep files focused — one topic area per file.

### Code Examples
- Fenced code blocks with language tags: ` ```go `, ` ```yaml `, ` ```bash `, ` ```json `, ` ```mermaid `.
- Use module path `stackyrd` for Go imports. Show complete snippets, not pseudocode.
- Include the `init()` function for registration patterns.

### Diagrams
- Mermaid for architecture (`flowchart`), state machines (`stateDiagram-v2`), interaction sequences (`sequenceDiagram`).
- Show key relationships, not every detail. Skip the diagram if a sentence covers it.

### Tables
- Use for reference data: endpoints, config fields, component lists. Clear headers, consistent alignment, logically sorted.

### Patterns by Doc Type

| Doc Type | Pattern |
|----------|---------|
| **Architecture** | Boot diagram → request flow → project structure → interfaces → registration → key features |
| **How-to / Development** | Step-by-step with code snippets. One `##` per extension point. |
| **Package deep-dive** | One `##` per sub-feature: description → usage → config → advanced. End with `## Best Practices`. |
| **Reference** | Tables. Config structure, endpoints, components, middleware. Group under `##` headings. |

## Workflow

| Action | Steps |
|--------|-------|
| **Add/Update** | Read source code (verify interfaces, config keys, imports) → update or create doc → update `docs_wiki/README.md` → check cross-references |
| **Remove** | Remove doc file → update README.md → update AGENTS.md if listed → fix cross-references |
| **Reorganize** | Preserve content, don't delete → update README.md → fix cross-references → update AGENTS.md |

## Common Mistakes

- Don't write docs without reading the source code.
- Don't forget to update README.md.
- Don't include auto-generated Swagger docs (`docs/` directory).
- Don't write duplicate content — link instead.
- Don't add diagrams that don't add clarity.

## Cross-references

- `stackyrd-dev` skill — use this skill when you need to understand the
  framework's extension patterns before writing docs about them.
- `AGENTS.md` — the canonical project policy about docs_wiki lives in the
  "Documentation" section near the end of the file.
