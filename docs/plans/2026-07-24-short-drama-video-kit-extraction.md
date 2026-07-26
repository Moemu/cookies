# Short Drama Video Kit Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the reusable, high-value reasoning patterns from the external short-drama video kit into the existing project-scoped short-drama pre-roll workflow, while preserving source rights, approval gates, and simulation-only delivery.

**Architecture:** Keep the current `BusinessTask -> GenerationJob -> Artifact -> ChangeSet` chain as the single source of truth. Add an auditable structured pre-roll planning layer that derives candidates from user-supplied, rights-cleared story inputs and approved Brief facts, persists the selected candidate and prompt snapshot in the existing video artifact, then continues to use the existing Ark generation job and approval path. Do not add direct video upload, video understanding, external object URLs, media composition, or a second client SDK in the MVP.

**Tech Stack:** TypeScript, React, Node.js test runner, existing file-backed MVP repository, Ark Provider.

---

## Research Basis

### Reviewed Inputs

- `/Users/bytedance/Downloads/short-drama-video-kit.ts`
- `/Users/bytedance/Downloads/说明.md`
- Existing [creative PRD](../02-creative-studio-prd.md), [media asset platform specification](../11-media-asset-platform.md), [API and event contract](../13-api-event-contracts.md), [cross-cutting requirements](../15-prd-cross-cutting-requirements.md), [video editor specification](../21-video-material-editor-spec.md), and [commerce pre-roll strategy](../策略/06-电商广告前贴与钩子视频生成策略.md).

### Current Project Fit

The project already has the necessary MVP backbone:

- `short_drama_preroll` is an existing project-scoped business task type.
- `purpose: "preroll"` plus `prerollType: "short_drama"` scopes jobs and artifacts.
- `GenerationJob` provides queued/running/succeeded/failed/cancelled transitions and provider polling.
- `SpecializedPages.tsx` provides the short-drama pre-roll workspace and only enables downstream use after a server-persisted artifact is ready.
- `ChangeSet` preflight prevents unapproved or non-ready creative assets from entering the simulation-only delivery path.

The extraction must strengthen this flow rather than import the external kit's independent `/uploads`, `/tasks`, `/media`, and `/creative` route family.

## Extraction Decision

| External pattern | Decision | Project adaptation | Reason |
| --- | --- | --- | --- |
| Framework-independent TypeScript helper layer | Adopt | Create a domain-only planner module with no React, DOM, provider, or fetch dependency | Keeps candidate selection deterministic and unit-testable |
| Async task polling with cancellation and terminal states | Keep existing | Continue using `GenerationJob` and provider polling | Existing state machine and persistence already cover this need |
| Structured episode data: summary, scenes, dialogue, emotion curve | Partially adopt | Accept a compact user-reviewed `story_context` JSON shape for short-drama pre-roll planning | Useful signal, but raw video understanding is not yet supported by the media platform |
| Candidate generation from conflict, reversal, and suspense | Adopt | Generate 3-5 explainable hook candidates with evidence and scoring reasons | Directly supports CR-022 and keeps operator choice visible |
| Prompt composition with visual style, narration, continuity, and guardrails | Adopt with changes | Build prompt from approved Brief/product facts plus selected candidate; prohibit invented claims and copied dialogue | Fits controlled ad generation rather than generic short-drama remixing |
| First-frame image then video generation | Defer | Record as P1 once image assets and stable Asset IDs are available | Current artifact content cannot safely reference generated image assets as media inputs |
| Script parsing into characters/scenes/props | Defer | Put behind the future media/document ingestion and rights pipeline | No document upload, parse job, evidence model, or source-rights workflow exists in MVP |
| Video upload and VLM understanding | Reject for this slice | Do not expose `/uploads/simple`, `/tasks`, or VLM video analysis routes | Conflicts with asset platform requirements for signed upload, scan, probe, derivatives, and Asset IDs |
| Emotion-curve high-light extraction | Defer | Revisit as `EditOperation` suggestion after video analysis and timeline metadata exist | Requires trustworthy timestamps, transcripts, and a non-destructive timeline |
| Multi-episode remix candidates and server composition | Reject for this slice | Keep output as a generated short pre-roll artifact only | Current MVP has no render worker, timeline version, rights-aware source segments, or legal remix clearance |
| Direct provider/public video URLs in output tracks | Reject | Persist controlled Artifact records only | The media platform explicitly prohibits long-lived provider URLs as business references |
| Client-held access token injection | Reject | Continue server-side Ark credentials only | `ARK_API_KEY` must never be exposed to the frontend |

## Target Slice

### In Scope

1. A deterministic, pure planner that creates short-drama pre-roll candidate cards from reviewed story context and approved Brief data.
2. Candidate evidence, score breakdown, narration, visual intent, continuity bridge, and a generation prompt with explicit factual/brand constraints.
3. A selected candidate snapshot stored in the existing generated video artifact content so the result remains reviewable after refresh.
4. A workspace UI that lets the operator inspect candidates, choose one, and submit only that candidate to the existing generation endpoint.
5. Server validation that only project-scoped short-drama work backed by a ready Brief can create a planning/generation request.
6. Tests for deterministic planning, invalid context, scope isolation, prompt safety, and preservation of current generation behavior.

### Explicitly Out of Scope

- Real ad-platform delivery, spend, or publishing.
- Browser upload of source videos, object-storage integration, VLM analysis, transcript extraction, and episode splitting.
- Automatic selection or automatic regeneration of a candidate.
- Copying reference video visuals, dialogue, music, faces, or brands.
- FFmpeg splitting/composition, editable time ranges, a render worker, or a new timeline schema.
- SSE streaming, first/last-frame references, and image-to-video chaining.
- A second media API namespace or a copied `ShortDramaVideoKit` SDK.

## Data Contract

Use a compact, reviewed planning input. It is intentionally not a claim that the system has analyzed the source video.

```ts
export type ShortDramaStoryContext = {
  title: string
  synopsis: string
  genre?: 'romance' | 'revenge' | 'suspense' | 'fantasy' | 'family' | 'other'
  approvedClaims: string[]
  openingLine?: string
  visualStyle?: string
}

export type ShortDramaPreRollCandidate = {
  id: string
  type: 'conflict' | 'reversal' | 'suspense' | 'benefit_bridge'
  title: string
  evidence: string
  score: number
  narration: string
  visualIntent: string
  continuityBridge: string
}

export type ShortDramaPreRollPlan = {
  storyContext: ShortDramaStoryContext
  candidates: ShortDramaPreRollCandidate[]
  generatedAt: string
  plannerVersion: 'v1'
}
```

Rules:

- `synopsis` and each `approvedClaims` item are operator-supplied/reviewed context, not model-extracted facts.
- The generator can only use approved claims, the project Brief, and the selected candidate as factual material.
- `openingLine` is only used to choose a non-verbatim bridge category. It must not be copied into narration or output prompt text.
- Candidate scoring is a transparent heuristic for ranking, not an effect forecast.
- The generated prompt must include channel, 9:16 aspect ratio, target duration, one primary action, static product/brand constraints, no copied source dialogue, no subtitles/text/watermarks unless a separately approved creative requirement requires them.

## File Structure

| File | Responsibility |
| --- | --- |
| `server/short-drama-preroll-planner.ts` | Pure story-context validation, candidate scoring, bridge selection, and prompt construction |
| `server/short-drama-preroll-planner.test.ts` | Unit tests for deterministic candidate generation and unsafe/malformed inputs |
| `server/domain.ts` | Adds only the typed pre-roll planning payload required by the API; retains existing task/job/artifact ownership |
| `server/generation-service.ts` | Validates short-drama scope and writes the selected planning snapshot into the resulting artifact content |
| `server/index.ts` | Exposes a project-scoped planning endpoint and extends existing media creation input without new provider routes |
| `server/preroll.test.ts` | Contract tests for Brief gate, scope isolation, selected-candidate persistence, and regressions |
| `src/data/api.ts` | Defines matching API DTOs and exposes a planning request method |
| `src/components/SpecializedPages.tsx` | Replaces fixed short-drama storyboard copy with server-returned candidates and explicit selection |
| `src/styles.css` | Styles the candidate list as a secondary panel without changing the existing primary preview flow |

## API Shape

Use the existing `/api` convention for the MVP, then migrate only during a broader API namespace migration. Do not introduce external-kit routes.

```http
POST /api/short-drama-preroll-plans
Content-Type: application/json
Idempotency-Key: optional-but-recommended

{
  "projectId": "project_xxx",
  "briefId": "artifact_xxx",
  "storyContext": {
    "title": "重生后她不再沉默",
    "synopsis": "女主发现被至亲利用，在关键公开场合反转局势。",
    "genre": "revenge",
    "approvedClaims": ["活动期间可领取新人券"],
    "openingLine": "那天我终于明白了真相。",
    "visualStyle": "写实都市短剧，克制冷色光"
  }
}
```

```json
{
  "storyContext": {},
  "candidates": [],
  "generatedAt": "2026-07-24T00:00:00.000Z",
  "plannerVersion": "v1"
}
```

The existing `POST /api/generation/media` request gains an optional `preRollPlan` and `selectedCandidateId` only when `purpose` is `preroll` and `prerollType` is `short_drama`. The server resolves the ID from the supplied plan, rebuilds the prompt server-side, and ignores a client-submitted raw generation prompt for this mode.

## Implementation Tasks

### Task 1: Add a Pure, Explainable Planner

**Files:**
- Create: `server/short-drama-preroll-planner.ts`
- Test: `server/short-drama-preroll-planner.test.ts`

- [ ] **Step 1: Write failing deterministic planner tests**

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { buildShortDramaPreRollPlan } from './short-drama-preroll-planner.js'

test('creates ranked, explainable candidates without copying the opening line', () => {
  const plan = buildShortDramaPreRollPlan({
    title: '重生后她不再沉默',
    synopsis: '女主发现被至亲利用，在关键公开场合反转局势。',
    genre: 'revenge',
    approvedClaims: ['活动期间可领取新人券'],
    openingLine: '那天我终于明白了真相。',
  })

  assert.ok(plan.candidates.length >= 3)
  assert.equal(plan.candidates[0]?.type, 'reversal')
  assert.match(plan.candidates[0]?.evidence ?? '', /反转/)
  assert.equal(plan.candidates.some((candidate) => candidate.narration.includes('那天我终于明白了真相')), false)
})
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `npx tsx --test server/short-drama-preroll-planner.test.ts`

Expected: FAIL because the planner module does not exist.

- [ ] **Step 3: Implement input validation and candidate construction**

```ts
export function buildShortDramaPreRollPlan(
  storyContext: ShortDramaStoryContext,
): ShortDramaPreRollPlan {
  const synopsis = storyContext.synopsis.trim()
  if (synopsis.length < 12) {
    throw new DomainError('VALIDATION_ERROR', '短剧梗概至少需要 12 个字符')
  }

  const candidates = rankCandidates(storyContext).slice(0, 5)
  if (candidates.length < 3) {
    candidates.push(...buildFallbackCandidates(storyContext, 3 - candidates.length))
  }

  return {
    storyContext: normalizeStoryContext(storyContext),
    candidates,
    generatedAt: new Date().toISOString(),
    plannerVersion: 'v1',
  }
}
```

Implement `rankCandidates()` so each candidate has an explicit keyword/evidence basis, a stable tie-breaker, and one candidate type per mechanism. Implement `buildContinuityBridge()` to map opening-line semantics to generic bridges such as `但真正的转折，才刚刚开始。`; never reuse source dialogue verbatim.

- [ ] **Step 4: Add safety-focused tests**

```ts
test('rejects missing approved claims and unsafe copied narration', () => {
  assert.throws(
    () => buildShortDramaPreRollPlan({
      title: '测试',
      synopsis: '这是一个足够长但没有通过审核卖点的故事梗概。',
      approvedClaims: [],
    }),
    /approvedClaims/,
  )
})
```

Add cases for whitespace normalization, a 5-candidate maximum, stable ordering, restricted candidate types, absent optional opening line, and no source-dialogue copying.

- [ ] **Step 5: Run the focused test**

Run: `npx tsx --test server/short-drama-preroll-planner.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit the isolated planner**

```bash
git add server/short-drama-preroll-planner.ts server/short-drama-preroll-planner.test.ts
git commit -m "feat: add short drama preroll planner"
```

### Task 2: Persist the Selected Candidate Through Existing Generation

**Files:**
- Modify: `server/domain.ts`
- Modify: `server/generation-service.ts`
- Modify: `server/repository.ts`
- Test: `server/preroll.test.ts`

- [ ] **Step 1: Write failing server contract tests**

```ts
assert.equal(response.status, 202)
assert.match(createdArtifact.content, /"selectedCandidate"/)
assert.equal(createdArtifact.prerollType, 'short_drama')
assert.equal(createdArtifact.projectId, project.id)
```

Create a sibling-project test that attempts to use another project's Brief or plan and expects `403` or the repository's established project-scope error.

- [ ] **Step 2: Run the focused contract test**

Run: `npx tsx --test server/preroll.test.ts`

Expected: FAIL because selected planning metadata is not persisted.

- [ ] **Step 3: Add typed generation metadata and server-side prompt rebuilding**

```ts
export interface ShortDramaPreRollGenerationInput {
  plan: ShortDramaPreRollPlan
  selectedCandidateId: string
}
```

In `generation-service.ts`, resolve `selectedCandidateId` against the submitted plan, rebuild the final prompt with `buildShortDramaPreRollPrompt()`, and create the job with the existing `purpose: 'preroll'` and `prerollType: 'short_drama'` fields. Store this JSON shape in the produced artifact `content`:

```ts
{
  "kind": "short_drama_preroll_v1",
  "selectedCandidate": {},
  "storyContext": {},
  "prompt": "server-generated prompt only",
  "plannerVersion": "v1"
}
```

Do not put provider task IDs, provider URLs, credentials, or raw user opening dialogue into the prompt snapshot.

- [ ] **Step 4: Enforce existing gates before job creation**

```ts
if (input.purpose === 'preroll' && input.prerollType === 'short_drama') {
  assertApprovedBrief(input.projectId, input.briefId)
  assertShortDramaPreRollInput(input.shortDramaPreRoll)
}
```

Use the repository's existing project and Brief validation path rather than duplicating permission logic. Reject a raw `prompt` override for short-drama pre-roll requests.

- [ ] **Step 5: Run focused server tests**

Run: `npx tsx --test server/preroll.test.ts server/provider.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit the persistence and gate changes**

```bash
git add server/domain.ts server/generation-service.ts server/repository.ts server/preroll.test.ts
git commit -m "feat: persist short drama preroll plans"
```

### Task 3: Expose a Project-Scoped Planning Endpoint

**Files:**
- Modify: `server/index.ts`
- Modify: `server/domain.ts`
- Test: `server/preroll.test.ts`

- [ ] **Step 1: Write failing endpoint tests**

```ts
const result = await request('/api/short-drama-preroll-plans', {
  method: 'POST',
  body: JSON.stringify({
    projectId: project.id,
    briefId: brief.id,
    storyContext,
  }),
})

assert.equal(result.status, 200)
assert.equal(result.body.plannerVersion, 'v1')
assert.ok(result.body.candidates.length >= 3)
```

Add a test for a missing/archived/unconfirmed Brief and expect `400` or the established validation error response.

- [ ] **Step 2: Run the endpoint test to verify it fails**

Run: `npx tsx --test server/preroll.test.ts`

Expected: FAIL with a missing route response.

- [ ] **Step 3: Add the route using shared request validation**

```ts
if (request.method === 'POST' && pathname === '/api/short-drama-preroll-plans') {
  const body = await readJsonBody(request)
  const projectId = requiredString(body, 'projectId')
  const briefId = requiredString(body, 'briefId')
  const storyContext = parseShortDramaStoryContext(body.storyContext)
  await repository.assertApprovedBriefForProject(projectId, briefId)
  return sendJson(response, 200, buildShortDramaPreRollPlan(storyContext))
}
```

Follow the error envelope already emitted by `server/index.ts`; do not expose model/provider internals or use a separate route prefix.

- [ ] **Step 4: Run focused server tests**

Run: `npx tsx --test server/preroll.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit the API contract**

```bash
git add server/index.ts server/domain.ts server/preroll.test.ts
git commit -m "feat: expose short drama preroll planning"
```

### Task 4: Replace Static Short-Drama Storyboard Copy With Candidate Selection

**Files:**
- Modify: `src/data/api.ts`
- Modify: `src/components/SpecializedPages.tsx`
- Modify: `src/styles.css`

- [ ] **Step 1: Add client DTOs and API method**

```ts
export type ApiShortDramaPreRollPlan = {
  storyContext: ApiShortDramaStoryContext
  candidates: ApiShortDramaPreRollCandidate[]
  generatedAt: string
  plannerVersion: 'v1'
}

createShortDramaPreRollPlan: (input: {
  projectId: string
  briefId: string
  storyContext: ApiShortDramaStoryContext
}) => request<ApiShortDramaPreRollPlan>('/short-drama-preroll-plans', 'POST', input),
```

- [ ] **Step 2: Add a reviewed-story-context form and loading/error states**

```tsx
const [storyContext, setStoryContext] = useState<ApiShortDramaStoryContext>(emptyStoryContext)
const [plan, setPlan] = useState<ApiShortDramaPreRollPlan | null>(null)
const [selectedCandidateId, setSelectedCandidateId] = useState<string | null>(null)
```

Require a ready current-project Brief before enabling planning. Show a recoverable error state that keeps the typed story context intact. Clearly label all suggestions as `AI 生成候选，需人工选择`.

- [ ] **Step 3: Render candidates as explainable, selectable cards**

```tsx
{plan?.candidates.map((candidate) => (
  <button
    key={candidate.id}
    type="button"
    aria-pressed={selectedCandidateId === candidate.id}
    onClick={() => setSelectedCandidateId(candidate.id)}
  >
    <strong>{candidate.title}</strong>
    <span>{candidate.evidence}</span>
    <small>建议机制：{candidate.type} · 评分：{candidate.score}</small>
  </button>
))}
```

Retain the current generated-video preview as the primary workspace. Candidate selection belongs in a collapsible secondary panel and must not create three equal-width columns.

- [ ] **Step 4: Submit only the selected plan through the existing generation route**

```tsx
await api.createPrerollVideo(scope, '', confirmedBriefId, {
  plan,
  selectedCandidateId,
})
```

Update the actual client API signature and request payload accordingly. For short drama, do not send an editable raw prompt. For game and commerce, retain current behavior unchanged.

- [ ] **Step 5: Build the frontend**

Run: `npm run build`

Expected: PASS.

- [ ] **Step 6: Commit the workspace change**

```bash
git add src/data/api.ts src/components/SpecializedPages.tsx src/styles.css
git commit -m "feat: select short drama preroll candidates"
```

### Task 5: Validate the Whole Flow and Document Deferred Work

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-07-24-short-drama-video-kit-extraction.md`

- [ ] **Step 1: Add a concise README capability note**

Document that short-drama pre-roll planning accepts operator-reviewed story context, generates explainable candidates, and produces a simulation-only creative artifact after normal approval gates. State that video upload, VLM analysis, remixing, and external ad delivery are not part of the current MVP.

- [ ] **Step 2: Run all mandatory checks**

Run:

```bash
npm run test:server
npm run check:server
npm run build
git diff --check
```

Expected: all commands exit with code `0`.

- [ ] **Step 3: Manually verify state gates**

1. Create a project and approved Brief.
2. Enter reviewed story context and request a plan.
3. Select one candidate and start generation.
4. Refresh while the job is running and confirm the job/artifact remains project-scoped.
5. Confirm a ready artifact can enter ChangeSet preflight.
6. Confirm delivery remains a local simulation and no third-party ad platform is called.
7. Try an unapproved Brief and a cross-project Brief; confirm both are rejected.

- [ ] **Step 4: Commit verification documentation**

```bash
git add README.md docs/plans/2026-07-24-short-drama-video-kit-extraction.md
git commit -m "docs: document short drama preroll workflow"
```

## Acceptance Criteria

1. The user can create at least three, at most five short-drama pre-roll candidates from reviewed story context and an approved current-project Brief.
2. Every candidate displays a mechanism, evidence, score, narration, visual intent, and non-verbatim continuity bridge.
3. The user selects exactly one candidate before generation; automatic candidate selection is not allowed.
4. The server rebuilds the short-drama prompt and persists the selected candidate snapshot in the resulting artifact.
5. A short-drama generation request with a missing/unapproved/cross-project Brief fails without creating a job.
6. Existing game and commerce pre-roll generation behavior remains unchanged.
7. Existing job polling, cancellation, ready-artifact filtering, ChangeSet preflight, audit, and simulation-only delivery remain the only downstream path.
8. No frontend code receives `ARK_API_KEY`, provider credentials, long-lived provider URLs, or direct object-storage identifiers.
9. The full server suite, server TypeScript check, frontend build, and whitespace check pass.

## Deferred Follow-Up: Media Intelligence Foundation

Do not begin this follow-up until the media asset platform's upload, Asset ID/Version, scanning, rights, proxy, and processing-job requirements are implemented.

| Follow-up capability | Required prerequisite | Future owning boundary |
| --- | --- | --- |
| Script/document parsing | Document Asset ingestion, parsing job, source-evidence storage, rights metadata | Media platform + Creative |
| Video understanding and transcript | Signed multipart upload, scan/probe, proxy, VLM job, transcript evidence | Media platform + Insights/Creative |
| Emotion-curve highlight proposal | Trusted timestamped transcript and `TimelineVersion` | Creative editing |
| Multi-episode remix | Rights-cleared source segments, non-destructive timeline, render worker, audit | Creative editing |
| First/last-frame generation | Generated image Asset persistence and reference-image rights | Creative production |
| Streaming recommendation | Durable async task/events with sequence/reconnect semantics | Shared platform |

## Risks And Controls

| Risk | Control |
| --- | --- |
| Candidate language invents marketing facts | Use only approved claims and Brief facts; require server-side prompt construction |
| Source dialogue is copied into generated content | Store opening line only for semantic bridge selection; test that it is never emitted verbatim |
| “High score” is interpreted as conversion prediction | Label score as heuristic mechanism relevance; omit performance claims |
| Short-drama source rights are unclear | Require reviewed source context and existing approval path; defer source-media ingestion |
| Two competing async/media architectures emerge | Reuse `GenerationJob`/`Artifact`; do not copy external routes or SDK |
| Provider temporary URLs leak into business records | Continue artifact persistence and avoid output-track URL contracts |
| Current delivery safety is weakened | Preserve ChangeSet preflight and local simulation-only execution without exceptions |

## Self-Review

- **PRD coverage:** This plan implements the current short-drama portion of CR-007/CR-022 without expanding beyond the existing MVP media boundary. It preserves CR-013, CR-014, CR-015, CR-024, and project-scope requirements.
- **Intentional gaps:** Source-video analysis, clip extraction, rendering, and remixing are explicitly deferred because their required asset, rights, and timeline foundations are absent.
- **Placeholder scan:** No implementation task depends on an unspecified route, owner, or safety policy; all new behavior is scoped to named files and existing entities.
- **Type consistency:** Planning input/output is consistently named `ShortDramaStoryContext`, `ShortDramaPreRollCandidate`, and `ShortDramaPreRollPlan`; persisted selected metadata is `ShortDramaPreRollGenerationInput`.
