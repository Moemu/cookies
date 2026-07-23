# Volcad Workflow Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the reusable Volcad proposal-to-strategy-to-creative workflow into the existing Go platform and React workspace, with server-side Ark invocation, TOS-backed generated assets, and a persisted Polaris Fresh demo proposal.

**Architecture:** Keep shared assets, projects, identity, provider jobs, generated intake, and TOS access in `internal/platform`. Add the advertising business workflow under `internal/systems/strategy` and `internal/systems/creative`, publishing business APIs under `/api/{system}/v1`; templates and demo data are versioned source assets, while generated strategies and creative briefs are project-owned database records. Generated images use the existing Provider Job and Generated Intake path so no business endpoint obtains or exposes provider URLs or TOS credentials.

**Tech Stack:** Go 1.26, MySQL migrations, standard-library HTTP server, existing Volcengine TOS SDK, Ark HTTP adapter, React 19, TypeScript, Vite, Vitest.

---

### Task 1: Local Configuration And TOS Composition

**Files:**
- Modify: `.env.example`
- Modify: `internal/platform/config/config.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `cmd/cookies-api/main.go`
- Test: `internal/platform/config/config_test.go`

- [ ] **Step 1: Write failing configuration tests**

```go
func TestFromLookupMapsLocalTOSAndArkTextConfiguration(t *testing.T) {
    cfg, err := FromLookup(mapLookup(map[string]string{
        "COOKIES_ENV": "local",
        "COOKIES_BLOB_PROVIDER": "tos",
        "COOKIES_TOS_ENDPOINT": "tos-cn-hongkong.volces.com",
        "COOKIES_TOS_REGION": "cn-hongkong",
        "COOKIES_TOS_ACCESS_KEY": "test-access-key",
        "COOKIES_TOS_SECRET_KEY": "test-secret-key",
        "COOKIES_TOS_ASSETS_BUCKET": "lensrhyme",
        "COOKIES_TOS_QUARANTINE_BUCKET": "lensrhyme-quarantine",
        "COOKIES_ARK_TEXT_API_KEY": "test-ark-key",
        "COOKIES_ARK_TEXT_MODEL": "doubao-seed-2-1-pro-260628",
    }))
    if err != nil { t.Fatal(err) }
    if cfg.Provider.ArkText.Model != "doubao-seed-2-1-pro-260628" { t.Fatal("text model was not loaded") }
}
```

- [ ] **Step 2: Run the focused test**

Run: `go test ./internal/platform/config -run TestFromLookupMapsLocalTOSAndArkTextConfiguration -v`
Expected: FAIL because `ArkText` is not defined.

- [ ] **Step 3: Add typed configuration and composition**

```go
type ArkText struct {
    APIKey  string
    Model   string
    BaseURL string
}

type Provider struct {
    ImageAdapter string
    TextAdapter  string
    ArkImage     ArkImage
    ArkText      ArkText
}
```

Map `COOKIES_ARK_TEXT_API_KEY`, `COOKIES_ARK_TEXT_MODEL`, and `COOKIES_ARK_TEXT_BASE_URL`; validate that `ark_text` is local-only and has a key/model. In `.env.example`, document `COOKIES_BLOB_PROVIDER=tos`, the user-supplied TOS endpoint/region/bucket variable names, CDN domain, and blank credential placeholders. Add a text adapter to the API composition root without reading any value from client requests.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/platform/config -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .env.example internal/platform/config cmd/cookies-api
git commit -m "feat: configure Ark text and TOS workflow"
```

### Task 2: Ark Text Adapter And Volcad Prompt Templates

**Files:**
- Create: `internal/platform/provider/ark_text_adapter.go`
- Create: `internal/platform/provider/ark_text_adapter_test.go`
- Create: `internal/systems/strategy/prompts/templates.go`
- Create: `internal/systems/strategy/prompts/templates_test.go`
- Modify: `cmd/cookies-api/main.go`

- [ ] **Step 1: Write Ark adapter and template tests**

```go
func TestArkTextAdapterSendsSystemAndUserMessages(t *testing.T) {
    // Assert POST /chat/completions, bearer authentication, configured model,
    // and normalized SynchronousResult without exposing the API key.
}

func TestBuildProposalStrategyMessagesIncludesComplianceAndRequiredJSON(t *testing.T) {
    messages := prompts.BuildProposalStrategyMessages(prompts.ProposalInput{
        Brand: "极地鲜生", Product: "深海鳕鱼柳",
        Compliance: []string{"禁用绝对化用语"},
    })
    if !strings.Contains(messages[0].Content, "资深品牌电商广告策划") { t.Fatal("missing strategy role") }
}
```

- [ ] **Step 2: Run the focused tests**

Run: `go test ./internal/platform/provider ./internal/systems/strategy/prompts -run 'TestArkTextAdapter|TestBuildProposal' -v`
Expected: FAIL because adapter/template packages do not exist.

- [ ] **Step 3: Implement normalized Ark text invocation and versioned templates**

```go
const TemplateVersion = "volcad-v1"

func BuildProposalStrategyMessages(input ProposalInput) []provider.TextMessage
func BuildCopyMessages(input ProposalInput, materialType string, count int) []provider.TextMessage
func BuildImagePrompt(input ProposalInput, variant int) string
func BuildVideoPrompt(input ProposalInput, variant int) string
```

Port the intent of Volcad's strategy, copy, image, video, ZIP proposal parsing, and selected-product prompts. Enforce JSON-only strategy/copy responses in system instructions, preserve compliance restrictions, and keep templates in source-controlled Go code with a declared version. Implement the adapter with only `COOKIES_ARK_TEXT_*` configuration and vendor-neutral output fields.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/platform/provider ./internal/systems/strategy/prompts -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/provider internal/systems/strategy/prompts cmd/cookies-api
git commit -m "feat: add Ark text adapter and ad prompt templates"
```

### Task 3: Persisted Proposal And Creative Workflow

**Files:**
- Create: `migrations/strategy/20260723090000_ad_proposals.up.sql`
- Create: `internal/systems/strategy/model.go`
- Create: `internal/systems/strategy/repository.go`
- Create: `internal/systems/strategy/mysql_store.go`
- Create: `internal/systems/strategy/service.go`
- Create: `internal/systems/strategy/service_test.go`
- Create: `internal/systems/creative/service.go`
- Create: `internal/systems/creative/service_test.go`

- [ ] **Step 1: Write failing service tests**

```go
func TestCreateProposalPersistsPolarisBriefAndAuditMetadata(t *testing.T) {
    // Create a project-bound proposal and assert template_version,
    // compliance constraints, selected product, and immutable revision.
}

func TestCreativePlanRequiresApprovedStrategy(t *testing.T) {
    // Assert image/video prompt planning fails before strategy approval.
}
```

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/systems/strategy ./internal/systems/creative -v`
Expected: FAIL because the system packages and tables do not exist.

- [ ] **Step 3: Add project-owned workflow records**

```sql
CREATE TABLE strategy_proposals (... project_id ..., input_json JSON, template_version VARCHAR(64), status VARCHAR(32), ...);
CREATE TABLE strategy_outputs (... proposal_id ..., strategy_json JSON, approved_at DATETIME(6), ...);
CREATE TABLE creative_plans (... project_id ..., strategy_output_id ..., image_prompt TEXT, video_prompt TEXT, ...);
```

Implement proposal create/get, strategy generation through `provider.Service.GenerateText`, explicit approval, and creative plan generation. Every query must include organization and project identity; use the template version and canonical input hash for idempotency. A creative plan may only be created from an approved strategy; it returns a prompt plus model alias, not an object-store URL.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/systems/strategy ./internal/systems/creative -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add migrations/strategy internal/systems/strategy internal/systems/creative
git commit -m "feat: persist proposal strategy and creative plans"
```

### Task 4: HTTP APIs And Provider Job Handoff

**Files:**
- Create: `internal/systems/strategy/http.go`
- Create: `internal/systems/creative/http.go`
- Create: `internal/systems/strategy/http_test.go`
- Create: `internal/systems/creative/http_test.go`
- Modify: `cmd/cookies-api/main.go`
- Modify: `api/openapi/platform-v1.yaml`

- [ ] **Step 1: Write HTTP contract tests**

```go
func TestStrategyGenerateRejectsMissingTextScope(t *testing.T) { /* expect 403 */ }
func TestCreativeImageRequestCreatesProviderJobFromApprovedPlan(t *testing.T) { /* expect 202 */ }
func TestCreativeAPIResponseNeverContainsTOSCredentialsOrArkKey(t *testing.T) { /* marshal response and assert secrets absent */ }
```

- [ ] **Step 2: Run focused tests**

Run: `go test ./internal/systems/strategy ./internal/systems/creative -run 'TestStrategyGenerate|TestCreativeImage|TestCreativeAPIResponse' -v`
Expected: FAIL until business routes are registered.

- [ ] **Step 3: Implement business HTTP surfaces**

```text
POST /api/strategy/v1/projects/{project_id}/proposals
POST /api/strategy/v1/projects/{project_id}/proposals/{proposal_id}/generate
POST /api/strategy/v1/projects/{project_id}/strategies/{strategy_id}/approve
POST /api/creative/v1/projects/{project_id}/plans
POST /api/creative/v1/projects/{project_id}/plans/{plan_id}/image-jobs
GET  /api/creative/v1/projects/{project_id}/plans/{plan_id}
```

Reuse request context, project authorization, provider scopes, project context version checks, idempotency keys, and `ProviderJobs.CreateImageJob`. The image job must reference the creative plan as `source_system=creative`, `source_task_id=<plan id>`, then rely on existing generated intake/TOS path for final assets. Publish the request/response schemas in OpenAPI.

- [ ] **Step 4: Run HTTP and contract tests**

Run: `go test ./internal/systems/strategy ./internal/systems/creative ./internal/platform/httpserver -v && make contract-check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/systems api/openapi cmd/cookies-api
git commit -m "feat: expose strategy and creative workflow APIs"
```

### Task 5: Seeded Polaris Fresh Proposal And TOS-Backed Demo Assets

**Files:**
- Create: `internal/systems/strategy/seed.go`
- Create: `internal/systems/strategy/seed_test.go`
- Create: `docs/demo/polaris-fresh-proposal.json`
- Create: `docs/demo/polaris-fresh-creative-brief.md`
- Modify: `cmd/cookies-api/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write a seed idempotency test**

```go
func TestSeedPolarisFreshProposalIsIdempotent(t *testing.T) {
    // Run seed twice for one local project; assert one proposal, one approved
    // strategy, and no duplicate creative plan are persisted.
}
```

- [ ] **Step 2: Run the focused test**

Run: `go test ./internal/systems/strategy -run TestSeedPolarisFreshProposalIsIdempotent -v`
Expected: FAIL because the seed is absent.

- [ ] **Step 3: Port safe mock data and document asset behavior**

Copy only the reusable proposal facts from Volcad: brand, product, target audience, platform, budget, compliance, creative directions, and 618 timeline. Do not copy proprietary source documents or embed binary images/videos. Seed the project-owned proposal, approved strategy, and image/video creative plans once. Document that generated remote media enters the existing generated intake flow and is stored in configured TOS assets bucket; local environment uses `.env`, never source-controlled credentials.

- [ ] **Step 4: Run seed tests**

Run: `go test ./internal/systems/strategy -run TestSeedPolarisFreshProposalIsIdempotent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/systems/strategy docs/demo README.md cmd/cookies-api
git commit -m "feat: seed Polaris Fresh creative demo"
```

### Task 6: Strategy And Creative Workspace UI

**Files:**
- Create: `web/src/features/strategy/api.ts`
- Create: `web/src/features/strategy/types.ts`
- Create: `web/src/features/strategy/StrategyWorkspacePage.tsx`
- Create: `web/src/features/strategy/StrategyWorkspacePage.test.tsx`
- Create: `web/src/features/creative/api.ts`
- Create: `web/src/features/creative/types.ts`
- Create: `web/src/features/creative/CreativeWorkspacePage.tsx`
- Create: `web/src/features/creative/CreativeWorkspacePage.test.tsx`
- Modify: `web/src/shell/Workspace.tsx`
- Modify: `web/src/styles.css`

- [ ] **Step 1: Write frontend route and user-flow tests**

```tsx
it('renders proposal status and sends approval from the strategy workspace', async () => {
  // Mock project-scoped Strategy API; assert generate and approve controls.
})

it('creates an image Provider Job only from an approved creative plan', async () => {
  // Assert disabled control for unapproved state and POST body for approved state.
})
```

- [ ] **Step 2: Run focused UI tests**

Run: `npm test --prefix web -- StrategyWorkspacePage CreativeWorkspacePage`
Expected: FAIL because pages do not exist.

- [ ] **Step 3: Add project-scoped workspaces**

Implement `/projects/:projectId/strategy` and `/projects/:projectId/creative`. Strategy displays the imported Polaris Fresh proposal, compliance boundary, template version, structured strategy output, generation state, and explicit approval. Creative displays image/video prompt variants, provenance/model aliases, and submits image jobs through the business API; it links final generated assets back to the existing project asset library. Use `apiRequest`, `AbortController`, and `ApiProblem`; never render server credentials or provider task handles.

- [ ] **Step 4: Run frontend checks**

Run: `npm run check --prefix web`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat: add strategy and creative workspaces"
```

### Task 7: Full Verification And Delivery

**Files:**
- Modify: `README.md`
- Modify: `.env.example`
- Modify: `docs/demo/polaris-fresh-creative-brief.md`

- [ ] **Step 1: Verify local configuration does not leak credentials**

Run: `git ls-files .env && git diff --check && git grep -n 'OBJECT_STORAGE_ACCESS_KEY_SECRET'`
Expected: no tracked `.env`, no whitespace error, and no actual secret in tracked files.

- [ ] **Step 2: Run backend quality gates**

Run: `make check`
Expected: Go formatting, vet, tests, Web checks, and OpenAPI contract checks pass.

- [ ] **Step 3: Run adversarial API probes**

```bash
curl -i -X POST /api/creative/v1/projects/<id>/plans/<id>/image-jobs
# Before approval: 409/400 business rejection.
# After approval with valid scope/context: 202 and no credentials/provider output URLs.
```

- [ ] **Step 4: Verify TOS configuration in local environment**

Run: `COOKIES_ENV=local go run ./cmd/cookies-api`
Expected: startup validates TOS environment variables without logging access key, secret, or signed URL.

- [ ] **Step 5: Commit and push**

```bash
git add README.md .env.example docs/demo
git commit -m "docs: document Volcad workflow migration"
git push origin codex/commerce-hook-workspace
gh pr checks --watch --interval 10
```
