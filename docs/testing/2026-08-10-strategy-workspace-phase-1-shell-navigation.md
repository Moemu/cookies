# Strategy workspace Phase 1 shell and navigation evidence

Date: 2026-08-10

Status: passed. Phase 1 is ready to gate into Phase 2; no commit, push, production migration, or deployment was performed.

## Scope delivered

- Replaced the nine workspace views with five URL-authoritative stages: `intake`, `brief`, `strategy`, `review`, and `handoff`.
- Added a Strategy-scoped shell, stable stage rail, workspace identity/switcher, contextual supporting panels, Activity entry, and a project AI assistant seam.
- Reused the existing persisted conversation, Brief, strategy, review, and creative handoff contents. The downstream image/video production flow was not changed.
- Consolidated the cross-workspace Demand, Research, Strategy, and Review centers into one prominent `策略中枢` sidebar group. This separates lifecycle execution in the five-stage workspace from cross-workspace asset reuse and centralized handling.
- Added one-time canonical redirects for legacy `?view=` workspace URLs.

## Navigation invariants

Canonical workspace route:

`/projects/{projectId}/strategy/workspaces/{workspaceId}/{stage}?panel={panel}&resource={resource}`

Legacy destinations are deterministic:

| Legacy view | Canonical destination |
| --- | --- |
| 概览 / 对话 | `intake` |
| Brief | `brief` |
| 研究 | `brief?panel=research` |
| 策略 | `strategy` |
| 实验 | `strategy` |
| 评审 | `review` |
| 变更记录 | `strategy?panel=history` |
| 创意任务策略 | `handoff` |

The URL is the stage source of truth. Browser back/forward restores the stage and supporting panel. A stage change restores that stage's own `.kanon-strategy-main` scroll position, focuses the stage heading without moving the window, and does not depend on background task completion.

Post-phase hardening completed the remaining `IA-07` input boundary across all five stages: unsent Intake content and attachment selections, open Brief field edits, Research questions/selections/proposal edits, Strategy section edits and natural-language revision instructions, Review comments/return reasons, and Handoff task answers survive stage, panel and Strategy-section switches. Recovery keys are versioned and scoped by organization, user, Project and Workspace; browser `sessionStorage` has an in-memory fallback and no value is written to metrics, logs or URLs. Strategy sections visibly mark unsaved drafts and block publish/submit until every draft is saved or discarded; Review and Handoff likewise block irreversible decisions or generation while transient input remains unresolved.

## Interaction and visual decisions

- The shell uses Strategy-local cobalt/cool-gray tokens and a responsive grid rather than adding more global page conventions.
- Supporting panels and the AI assistant are mutually exclusive, preventing a three-column squeeze at desktop widths.
- At 820px, supporting panels and the assistant become overlays and do not create body overflow.
- The AI assistant uses the current persisted workspace conversation and remains available across the five stages. It does not claim that Phase 3 memory, proposals, unread state, or resizable docking already exists.
- The Activity panel reports only known current tasks. It does not fabricate percentages or claim heartbeat/stalled/retry support before Phase 2.
- `策略中枢` presents Demand → Research → Strategy → Review in lifecycle order, with short responsibility labels. At the 1279px breakpoint it degrades to compact icons with stable accessible names.

## Browser verification

Environment:

- Local Vite application: `http://127.0.0.1:5173`
- Local Go API: `http://127.0.0.1:8080`
- Project: `project_dee069688be97141ad7ba891e97622f0`
- Workspace: `strategyws_4cec9f802ea12cbf2ac8c7b155db2e14`

Checks performed:

| Check | Result |
| --- | --- |
| Five stages visible and truthful status labels | Passed |
| Brief and Strategy stage URL/focus/scroll behavior | Passed |
| Research contextual panel deep link | Passed |
| AI assistant remains available across stage changes | Passed |
| Browser back/forward restores stage | Passed |
| Legacy `?view=研究` canonical redirect | Passed |
| 820px research and assistant overlays | Passed; no body overflow |
| Strategy hub expanded state at 1440px | Passed; four 52px center entries with responsibility labels |
| Strategy hub compact state at 1200px | Passed; 64px sidebar, no body overflow, accessible names preserved |
| Application console errors/warnings | None |
| Per-stage scroll and unsaved input recovery across Intake/Brief/Strategy/Review/Handoff plus the Research panel | Passed in the current 7/7 real-browser suite |

Reference captures from the workspace QA run are stored outside the repository:

- `C:\Users\Admin\AppData\Local\Temp\strategy-phase1-desktop.png`
- `C:\Users\Admin\AppData\Local\Temp\strategy-phase1-narrow.png`

## Automated gates

| Gate | Result |
| --- | --- |
| `go test ./...` | Passed |
| `npm test` | 156 passed, 0 failed |
| `npm run contract:check` | 7 frontend contract tests plus Go contract/integration packages passed |
| `npm run build` | Passed; 1,825 modules transformed |
| `git diff --check` | Passed |
| UTF-8/trailing whitespace scan for untracked text artifacts | Passed |
| `gofmt -l` for changed/untracked Go files | Passed |

Build evidence:

- Strategy workspace lazy JS: 135.98 kB, gzip 41.23 kB.
- Strategy workspace lazy CSS: 16.23 kB, gzip 3.26 kB.
- Main JS: 1,114.31 kB, gzip 314.96 kB.
- Global CSS: 439.01 kB, gzip 72.89 kB.
- Vite still reports the pre-existing main-chunk-over-500-kB warning. The Strategy workspace remains lazy-loaded; broad global bundle splitting is deferred as a separate performance task.

## Adversarial review

1. No second stage source of truth was introduced; React renders the parsed canonical URL.
2. No new polling loop, fake progress, model write path, schema bypass, or background-state-driven navigation was introduced.
3. Refresh and browser history reconstruct stage/panel state. Existing workspace data remains server-persisted.
4. The assistant seam reuses the existing conversation and does not label inferred context as verified fact.
5. Activity is a snapshot only; it explicitly defers heartbeat, stalled detection, retry-stage, and SSE reconciliation to Phase 2.
6. No cross-project resource lookup or new sensitive data in URLs was added. Only IDs, stage, panel, and optional selected-resource IDs are routable.
7. No Creative contract or downstream production behavior was modified.
8. Visual QA found and fixed two issues before acceptance: simultaneous supporting/assistant panels at desktop width, and missing compact-mode accessible names for prominent hub entries.
9. A 1440px explicit test viewport exposed an existing global topbar overflow when every topbar control is visible. It is outside the Strategy sidebar change and remains a separate shell-level responsive defect; the 1200px supported responsive state has no overflow.

## Deferred gates

- Phase 2: unified Activity projection/API/SSE, heartbeat, stalled detection, stage-specific retry, and freeze recovery.
- Phase 3: reconstructible project context manifest, assistant memory compaction, proposal acceptance, unread state, resize, and full cross-stage assistant behavior.
- Phase 4 and later: asynchronous deep research loop/adoption, StrategyDraft v3 persistence, review role rules, document parse progress/quality/vision fallback, metrics rollout, and production migration.
