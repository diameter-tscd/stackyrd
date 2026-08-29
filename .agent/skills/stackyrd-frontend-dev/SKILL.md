---
name: stackyrd-frontend-dev
description: Develop and maintain the SvelteKit admin dashboard in web/ — Svelte 5 + SvelteKit 2 + Tailwind v4 + Vite 8 + shadcn-svelte SPA that manages the stackyrd Go service framework. Use this skill whenever the user works on web/ files, adds or modifies Svelte routes, components, stores, API clients, or styles; touches web/package.json, vite.config.ts, svelte.config, tsconfig.json, or src/lib and src/routes; mentions dashboard, frontend, SvelteKit, Svelte 5 runes, Tailwind, shadcn, theme, Vite proxy, MCP, or PUBLIC_API_URL; or runs web dev/build/check commands. Even if the user does not name the skill explicitly, apply it for any web/ dashboard task.
---

# stackyrd Frontend Dev Guide

SvelteKit + shadcn-svelte admin dashboard in `web/` — SPA for stackyrd Go service framework (Echo v4). Playful bold design (black 2px borders, rounded-full/2xl), polling + MCP JSON-RPC + REST.

```
web/ → Vite proxy (/api,/mcp,/health,/metrics → :8080) → stackyrd Go → SvelteKit SPA
```

## Stack

| Layer | Choice | Version |
|-------|--------|---------|
| Framework | SvelteKit | 2.63 |
| UI | Svelte 5.56 runes + shadcn-svelte + bits-ui | 5.56 |
| Build | Vite 8 | 8.0 |
| Style | Tailwind v4 + tw-animate-css (`@import "tailwindcss"`) | 4.x |
| Variants | tailwind-variants, clsx, tailwind-merge | — |
| Icons | lucide-svelte only (never emoji) | 0.460 |
| Fonts | Space Grotesk (sans/display), Space Mono (mono) | Google Fonts |
| Lang | TypeScript 6 strict | 6.0 |
| Adapter | @sveltejs/adapter-auto | 7.0 |

No emoji as icons — always `lucide-svelte` SVG. One file before three, stdlib before deps.

## Quick Reference

```bash
cd web && npm install
cp .env.example .env  # PUBLIC_API_URL, PUBLIC_MCP_URL, PUBLIC_API_TOKEN
npm run dev     # Vite dev proxy → :8080 (no CORS)
npm run build   # vite build
npm run check   # svelte-kit sync + svelte-check
npm run preview
```

## Directory Layout

```
web/
├── vite.config.ts      # sveltekit() + tailwindcss() + server.proxy (/api,/mcp,/health,/metrics)
├── svelte.config.js    # adapter-auto + $lib alias (required for shadcn)
├── components.json     # shadcn config (zinc, $lib/components, $lib/utils)
├── tsconfig.json       # extends .svelte-kit/tsconfig.json, strict
├── static/robots.txt
├── src/
│   ├── app.html        # no hardcoded class; inline script hydrates theme from localStorage before paint
│   ├── app.css         # p5-inspired tokens: @theme inline + :root/.dark/.night
│   ├── app.d.ts        # PUBLIC_API_* decls
│   ├── lib/
│   │   ├── utils.ts        # cn(...inputs) → twMerge(clsx(...))
│   │   ├── api/
│   │   │   ├── rest.ts         # apiFetch<T>(path, token, opts) — browser uses relative via proxy
│   │   │   ├── mcp.ts          # resolveMcpUrl(): localStorage mcpUrl or /mcp (browser) / PUBLIC_MCP_URL (SSR), POST + MCP-Protocol-Version
│   │   │   └── endpoints.ts    # typed wrappers + normalization + PUBLIC_API_TOKEN fallback
│   │   ├── stores/
│   │   │   ├── auth.ts         # auth, theme: ThemeName light|dark|night + applyTheme(), toasts, sidebarCollapsed
│   │   │   └── data.ts         # health, services, infra, endpoints, connectionStatus
│   │   ├── types/api.ts        # ApiResponse, HealthData (flexible), ServiceMeta, InfraStatus, EndpointList
│   │   └── components/
│   │       ├── layout/         # Sidebar (w-64), TopBar (h-16), PageHeader (p5 header)
│   │       └── ui/             # shadcn: Button (rounded-full), Card (border-2), Badge, Input, Label, Separator, Table, Skeleton, Alert, ThemeToggle, ConnectionAlert, EmptyState (icon: Component), StatCard, Spinner, Toast
│   └── routes/
│       ├── +layout.svelte      # import '../app.css', auth guard, <Sidebar/><TopBar/><main pt-16 pl-64|pl-16><ConnectionAlert/>
│       ├── +page.svelte        # Overview — Promise.allSettled health/services/infra, uptimeDisplay fallback
│       ├── login/+page.svelte  # shadcn Card + Input/Label
│       ├── services/+page.svelte + [name]/+page.svelte
│       ├── infrastructure/+page.svelte + [name]/+page.svelte
│       ├── metrics/+page.svelte
│       ├── logs/+page.svelte   # EventSource /api/v1/events/stream
│       ├── endpoints/+page.svelte
│       ├── config/+page.svelte
│       └── settings/+page.svelte # Appearance: single ThemeToggle
```

## Critical Fixes (do not regress)

### CSS Not Loading
`+layout.svelte` must `import '../app.css'` — without it Tailwind/shadcn tokens never load → plain HTML.

### CORS / Vite Proxy
`rest.ts` uses `browser ? path : PUBLIC_API_URL+path` — browser fetches `"/health"` via proxy (same-origin), SSR uses absolute. `mcp.ts` uses `resolveMcpUrl()`: browser → `localStorage.mcpUrl ?? "/mcp"` via proxy, SSR → `PUBLIC_MCP_URL`. Never use absolute `PUBLIC_*` in browser fetch. Proxy in `vite.config.ts` handles all 4 prefixes.

### MCP GET Not Supported
MCP endpoint is POST-only. `mcp.ts` always `POST` with `Content-Type: application/json`, `Accept: application/json, text/event-stream`, `MCP-Protocol-Version: 2025-03-26`. `resolveMcpUrl` prevents `GET /mcp` from Vite. `ApiError` handled as `405`.

### Health/Spinner Forever + Missing Services/Infra/Uptime
`Promise.all` failing on one MCP `401` blocked `health.set()`. Now `Promise.allSettled` per-resource, `connectionStatus` set `connected` if any success. `endpoints.ts` normalizes raw MCP shapes: `services` (`state`→`status`, `endpoints: string[]`→`EndpointMeta[]`), `infra` (`status.connected`→string), `endpoints: string[]`→`EndpointList` grouped, `health` fallback `uptimeDisplay = uptime ?? status ?? progress%`. Token fallback: `effToken(token) => token || PUBLIC_API_TOKEN`.

### Theme Flash
`app.html` inline script reads `localStorage stackyrd_theme` before paint and sets `html.classList + colorScheme`. `auth.ts` `theme` is `ThemeName` and `applyTheme()` on subscribe.

### Layout Stacking
`Sidebar` `w-64` expanded / `w-16` collapsed, `TopBar` `left-64`/`left-16` `h-16`, `main` `pt-16 pl-64|pl-16`. All three must stay in sync — previously `pl-56` caused 32px overlap.

### Emojis Banned
`EmptyState` prop is `icon?: unknown` (lucide Component), usage `icon={Search}` not `icon="🔍"`. Search codebase for `icon="[^"]"` and `📊🔗⚙️🔌🔍⚠️` before commit.

## Conventions

### Svelte 5 Runes
```svelte
<script lang="ts">
  import type { Snippet } from 'svelte';
  let { label, value, children, class: className = '' } = $props<{ label: string; value: string; class?: string; children?: Snippet }>();
  let filter = $state<'all' | 'running'>('all');
  let loading = $state(true);
  const filtered = $derived(filter === 'all' ? $services : $services.filter(s => s.status === filter));
  $effect(() => { if (!$auth.authenticated && $page.url.pathname !== '/login') goto('/login'); });
  onMount(async () => { await pollData(); polling = setInterval(pollData, 10000); });
  onDestroy(() => { if (polling) clearInterval(polling); });
</script>
```
`$props` with `class: className`, `$state`/`$derived`/`$effect`, `{@render children?.()}`, `onclick` attribute, `Snipper` not `<slot>`.

### Routing
`+page.svelte` per segment, `[name]/+page.svelte` dynamic, `+layout.svelte` shell with auth guard (`onMount` + `$effect` → `/login`). Sidebar active via `RegExp` tested against `$page.url.pathname`.

### Stores
```ts
// auth.ts — ThemeName light|dark|night, persisted, applied to html
export const theme = writable<ThemeName>(loadTheme()); // applyTheme() sets html class
export const auth = createAuthStore(); // token, apiUrl, mcpUrl, authenticated, lastChecked
export const toasts = createToastStore(); // add(type, msg, duration?), dismiss, clear
export const sidebarCollapsed = writable(false);

// data.ts
export const health = writable<HealthData | null>(null);
export const services = writable<ServiceMeta[]>([]);
export const infra = writable<InfraStatus[]>([]);
export const connectionStatus = writable<'connected'|'disconnected'|'checking'>('checking');
```
Guard `localStorage`/`crypto.randomUUID()` with `browser`. `hasNotified` in `connection-alert` dedupes toast.

### API Layer
```ts
// rest.ts
apiFetch<T>(path, token, {method, body, headers, signal}): Promise<T> // browser→relative via proxy
// mcp.ts
resolveMcpUrl(endpoint?) // browser: localStorage.mcpUrl ?? "/mcp", server: PUBLIC_MCP_URL
mcpCall(token, method, params?, endpoint?): Promise<unknown> // POST + MCP-Protocol-Version
// endpoints.ts — normalization + effToken fallback
getHealth, getMCPServices (raw→ServiceMeta), getMCPInfra (raw→InfraStatus), getMCPEndpoints (string[]→EndpointList), getMetrics
```
`Authorization: Bearer <token>` when present. `mcp.ts` parses `text`→`JSON.parse` then `data.error` check, `405 GET` friendly message.

### Styling (Tailwind v4 + shadcn)
`app.css`:
```css
@import "tailwindcss"; @import "tw-animate-css";
@custom-variant dark (&:is(.dark *, .night *)); @custom-variant night (&:is(.night *));
@theme inline { --color-background: var(--background); ... --color-p5-yellow: #dfed33; --color-p5-magenta: #ed225d; --font-sans: 'Space Grotesk',...}
:root  /* light */  { --background:#fff; --foreground:#000; --card:#fff; --primary:#ed225d; --secondary:#f7f7f7; --muted:#f7f7f7; --border:#000; --sidebar:#fff; }
.dark  /* dark */   { --background:#020617; --foreground:#f9b2c6; --card:#111; --primary:#f1678e; --border:#2a2a2a; }
.night /* OLED */   { --background:#000; --foreground:#fafafa; --card:#09090b; --primary:#000; --border:#000; }
@layer base { *{@apply border-border} body{@apply bg-background text-foreground} ::selection{background:var(--color-p5-yellow)} }
```
- Light is default, dark = p5 dark (black bg magenta type), night = monochrome true-black.
- Components: `Button` `rounded-full border-2 border-black`, `Card` `rounded-2xl border-2 border-black shadow-sm`, `Input` `rounded-full border-2`, `Badge` `rounded-full border-2`, `StatCard` top `h-1.5` accent bar (yellow/magenta/green/blue).
- Sidebar `border-r-2 border-black`, active nav `bg-p5-yellow border-black rounded-full`, TopBar `h-16 border-b-2`, PageHeader `text-3xl md:text-4xl` + pink dot + mono subtitle `border-l-2`.
- Always `cn()` from `$lib/utils.ts`, `tailwind-variants` for Button/Badge, never raw hex in components.

### Environment
| Var | Source | Default |
|-----|--------|---------|
| `PUBLIC_API_URL` | `.env` | `http://localhost:8080` |
| `PUBLIC_MCP_URL` | `.env` | `http://localhost:8080/mcp` |
| `PUBLIC_API_TOKEN` | `.env` | `f76fda...` (fallback when `auth.token` empty) |

## Adding Things
### New Page
1. Create `routes/<name>/+page.svelte` with `onMount` poll + `onDestroy` + `$derived` filter (copy `services/+page.svelte` Promise.allSettled pattern).
2. Add to `Sidebar.svelte` `navItems` with lucide icon + `match: RegExp`.
3. Add type to `types/api.ts` if needed.
4. Add wrapper to `endpoints.ts` with `effToken` + normalization.
5. Use `PageHeader`, `Card`/`CardHeader`/`CardContent`, `Badge`, `Button`, `EmptyState icon={Lucide}`.

### New UI Component
Follow shadcn shape: `$props<{ variant?, size?, class?, children?: Snippet, [key:string]: unknown }>` + `cn()` + `tv({base, variants})`. Place under `ui/` with `components.json` aliases. Keep `utils.ts` `cn`.

### New Store
Ephemeral → `data.ts`, persisted → `auth.ts` (browser guard, `applyTheme` pattern).

## Ponytail Constraint
One file before three, delete before add. No new UI lib/state lib/chart lib unless task impossible with stores + Tailwind/shadcn. Keep `adapter-auto` + proxy + `PUBLIC_*` sufficient.

## Verification
```bash
cd web && npm run check  # 0 errors
cd web && npm run build  # vite build OK
```
If `$props` errors, run `svelte-kit sync` (`npm run check` does).
