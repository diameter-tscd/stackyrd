---
name: stackyrd-improver-check
description: >
  Analyze the stackyrd Go/Gin framework codebase for performance bottlenecks,
  code issues, enhancement opportunities, and native dependency risks. Use this
  skill whenever a user asks about improving stackyrd — whether they mention
  profiling, optimization, code review, bug hunting, dependency audit, security
  hardening, or architectural improvements. Also trigger when the user asks
  "what's wrong with", "how can we improve", "is this production-ready", "audit
  the codebase", "check for issues", "performance review", or similar diagnostic
  or evaluative questions about the stackyrd project. This skill is the go-to
  for any analysis, review, or improvement-planning task.
---

# stackyrd Improver & Check

**Before analyzing anything, first load and apply the [ponytail](../ponytail/SKILL.md) skill.** Ponytail governs analysis too: find the biggest real problems first, don't write exhaustive checklists of theoretical issues, skip empty sections, shortest report that captures actual impact.

Apply the ladder:
1. Does this analysis need to exist? If the code is fine, say so in one line.
2. Can one grep find the biggest issue? Start there.
3. Is the shortest report (3-5 findings, no empty sections) enough?
4. Only then: expand to more depth if real issues found.

## Approach

Run these four checks. **Stop early** if nothing critical found in a category — no empty sections in the report.

**Performance** — grep for `context.Background()` in production code, `time.After()` leaks, unbounded goroutines, unbuffered channels. Fix the real leaks, skip theoretical ones.

**Code Issues** — check for error swallowing (defer `_ =`), type assertion panics, config tag collisions (`mapstructure`), missing test coverage. Skip anything that compiles and works.

**Enhancements** — only if real issues found above. Prioritize by actual impact (per-request hot path > startup path).

**Native Deps** — check `CGO_ENABLED=0` builds, `go.mod` for CGO-heavy deps, platform build tags. If CI builds clean, say so and stop.

## Report Format

Shortest meaningful report. Skip empty sections entirely.

```markdown
# stackyrd Analysis Report

## Summary
<2-3 sentences max>

## Findings
### Critical
- **Location**: file:line — what, why it matters, fix

### Warning
- ...

### Info
- ...
```

Each finding: file:line, what, impact, fix. Read the actual code — don't report from grep alone.
