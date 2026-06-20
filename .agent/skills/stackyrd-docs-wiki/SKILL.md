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

Keep the hand-written documentation in `docs_wiki/` accurate, complete, and
in sync with the codebase. The `docs_wiki/` folder is the canonical source of
project documentation, indexed by `docs_wiki/README.md`.

## When This Skill Should Run

1. **Code-driven updates** — when a service, middleware, infrastructure
   component, plugin, or utility package is added, changed, or removed, the
   corresponding doc file must be updated or created.
2. **Explicit doc requests** — when the user says "update the docs for X",
   "write documentation for Y", "fix the README", or "docs_wiki is out of
   date".
3. **TOC maintenance** — when files in docs_wiki/ are added, renamed, or
   removed and the README.md index needs updating.

## Key Files

| Path | Purpose |
|------|---------|
| `docs_wiki/README.md` | Table of contents — must be updated whenever files in docs_wiki/ change |
| `docs_wiki/{TOPIC}.md` | Individual documentation files, one per topic area |
| `AGENTS.md` (lines 404-408) | Canonical policy: docs_wiki is the source of truth; update both doc and README when code changes |

## Documentation Conventions

All docs_wiki files follow these consistent conventions. Match them exactly.

### Structure
- Start every file with a `# Title` (H1) and a one-sentence summary line.
- Use `##` for major sections, `###` for subsections.
- Technical, professional tone. No emojis. No conversational language.
- Keep files focused — one topic area per file.

### Code Examples
- Always use fenced code blocks with language tags: ` ```go `, ` ```yaml `,
  ` ```bash `, ` ```json `, ` ```mermaid `.
- Go code examples must use the module path `stackyrd` for imports.
- Show complete, compilable-looking snippets — not pseudocode.
- For registration patterns, always include the `init()` function.

### Diagrams
- Use Mermaid `flowchart TD` (top-down) or `flowchart LR` (left-right) for
  architecture, boot sequence, request flow, and project structure diagrams.
- Use Mermaid `stateDiagram-v2` for state machines (e.g., circuit breaker).
- Diagrams should show key components and their relationships, not every
  implementation detail.

### Tables
- Use tables for reference data: endpoints, config fields, component lists,
  command references.
- Tables should have clear column headers and consistent alignment.
- Sort entries logically (alphabetically or by function).

### Patterns by Doc Type

**Architecture docs** (e.g., ARCHITECTURE.md):
- Boot sequence diagram → request flow → project structure tree →
  interface definitions → registration patterns → key features list.

**How-to / development docs** (e.g., DEVELOPMENT.md):
- Step-by-step instructions with code snippets for each task.
- Each extension point gets its own `##` section with a full code example,
  config toggle, and any extra steps.

**Package deep-dives** (e.g., RESILIENCE.md):
- One `##` section per sub-feature.
- Each sub-feature: brief description → basic usage → custom config →
  advanced usage.
- End with `## Best Practices`.

**Reference docs** (e.g., REFERENCE.md):
- Table-heavy — config structure, endpoint lists, component tables,
  middleware tables, command references.
- Group related tables under `##` section headings.

## Workflow: Adding or Updating Docs

When code changes affect documentation:

1. **Identify the doc file** — check if a file already exists for the topic
   (e.g., a new `pkg/caching/` package maps to `CACHING.md`). If not, you'll
   need to create one.
2. **Read the existing doc** (if any) to understand current content and style.
3. **Read the actual code** — don't write docs from memory. Open the relevant
   source files to verify interfaces, config keys, registration patterns, and
   import paths.
4. **Update or create the doc file** following the conventions above. Ensure
   all code examples, config keys, and interface signatures are accurate.
5. **Update `docs_wiki/README.md`** — add/remove/modify the table entry.
   - If adding a new doc to the "Package Deep Dives" section, include the
     package path in the table (e.g., `pkg/caching/`).
   - Keep the table formatting consistent with existing entries.
6. **Check if any other doc files need updating** — e.g., adding a new
   middleware might require updates to both SECURITY.md and REFERENCE.md.

## Workflow: Removing Docs

When a package or feature is removed:

1. **Remove the doc file** from docs_wiki/ (or merge its content into a
   related doc if the removal is partial).
2. **Update `docs_wiki/README.md`** — remove the table row.
3. **Update `AGENTS.md`** — remove the entry from the directory tree listing
   if it's listed there.
4. **Check cross-references** — other doc files may link to the removed doc.
   Update or remove those links.

## Workflow: Reorganizing Docs

When renaming or splitting docs:

1. **Preserve content** — don't delete useful information, just move it.
2. **Add redirect notes** — if renaming a file, the old name can briefly
   contain a note pointing to the new location (but don't leave this
   permanently).
3. **Update `docs_wiki/README.md`** fully — change the link, file name, and
   description.
4. **Update cross-references** across all other doc files.
5. **Update `AGENTS.md`** if the file is referenced in the directory tree.

## Common Mistakes to Avoid

- **Don't write docs without reading the source code** — interface signatures
  and config keys drift. Always verify.
- **Don't forget to update README.md** — it's the index. If it's wrong, the
  entire doc tree is effectively lost.
- **Don't use generic descriptions** — every README.md table entry should
  succinctly describe what the doc covers.
- **Don't include auto-generated Swagger docs** (`docs/` directory) — those
  are generated by `scripts/swagger/swagger.go` and managed separately.
- **Don't invent Mermaid diagrams** for things that don't benefit from
  visualization — use them for architecture, state machines, and flows where
  a diagram genuinely adds clarity.
- **Don't write duplicate content** — if a package is already documented in
  another file, link to it rather than repeating.

## Cross-references

- `stackyrd-dev` skill — use this skill when you need to understand the
  framework's extension patterns before writing docs about them.
- `AGENTS.md` — the canonical project policy about docs_wiki lives in the
  "Documentation" section near the end of the file.
