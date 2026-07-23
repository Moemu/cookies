# Investor MVP Task 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the local MVP repeatable for an investor demo, including seeded data, guided navigation, safe Provider degradation, accurate setup documentation, and verified quality gates.

**Architecture:** The repository owns an idempotent demo seeder so the default server process creates one durable project with confirmed upstream artifacts, a preflight-passed ChangeSet, and auditable events. The React app derives a clear five-step route guide from this project and reads audit events through the existing server API. Provider status remains server-derived; no browser secret is introduced.

**Tech Stack:** TypeScript, Node.js `http`, JSON-file repository, React, Vite, Node.js `node:test`.

---

### Task 1: Seed a Repeatable Demo

**Files:**
- Create: `server/demo.ts`
- Modify: `server/index.ts`
- Modify: `server/test/task1.test.ts`

- [ ] **Step 1: Write a failing idempotency test**

```ts
await seedDemoProject(repository);
await seedDemoProject(repository);
assert.equal((await repository.listProjects()).length, 1);
assert.equal((await repository.listChangeSets(project.id))[0]?.status, "preflight_passed");
```

- [ ] **Step 2: Implement an idempotent seed**

```ts
export async function seedDemoProject(repository: FileRepository): Promise<Project> {
  const existing = (await repository.listProjects()).find(
    (project) => project.name === DEMO_PROJECT_NAME,
  );
  if (existing) return existing;
  // Create a confirmed brief, ready image asset, ChangeSet, and preflight audit trail.
}
```

- [ ] **Step 3: Seed only the normal local server startup**

```ts
const repository = await FileRepository.open(resolve(process.cwd(), "data/mvp-store.json"));
await seedDemoProject(repository);
createApp({ repository }).listen(port, "127.0.0.1");
```

- [ ] **Step 4: Run the focused test**

Run: `npm run test:server`
Expected: all server tests pass.

### Task 2: Guide the Demo and Show Audit Evidence

**Files:**
- Modify: `src/data/api.ts`
- Modify: `src/components/Pages.tsx`
- Modify: `src/styles.css`

- [ ] **Step 1: Add a typed audit-event client**

```ts
export interface ApiAuditEvent {
  id: string;
  projectId: string;
  actor: string;
  action: string;
  createdAt: string;
}

listAuditEvents: (projectId: string) =>
  request<ApiAuditEvent[]>(`/api/audit-events?projectId=${encodeURIComponent(projectId)}`),
```

- [ ] **Step 2: Add a five-step route guide to the project dashboard**

```tsx
const demoSteps = [
  { label: "需求与策略", system: "strategy", navId: "workspaces" },
  { label: "创意产物", system: "creative", navId: "image-text" },
  { label: "投放预检", system: "delivery", navId: "plans" },
  { label: "人工审批", system: "delivery", navId: "approvals" },
  { label: "审计记录", system: "delivery", navId: "evidence" },
] as const;
```

- [ ] **Step 3: Render service-backed audit evidence for the delivery evidence page**

```tsx
const [events, setEvents] = useState<ApiAuditEvent[]>([]);
useEffect(() => {
  void api.listAuditEvents(currentProject.id).then(setEvents).catch(() => undefined);
}, [currentProject.id]);
```

- [ ] **Step 4: Make Provider degradation explicit**

```tsx
{configuredCount === 0 ? (
  <div role="status">未配置方舟 Provider：仍可完成预置项目讲解；AI 生成按钮保持禁用。</div>
) : null}
```

- [ ] **Step 5: Run frontend build**

Run: `npm run build`
Expected: exit code 0.

### Task 3: Document Local MVP Boundaries

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `.env.example`
- Modify: `.trae/specs/build-investor-mvp/tasks.md`

- [ ] **Step 1: Replace obsolete “not implemented” status with runnable MVP setup**

```bash
cp .env.example .env
npm install
npm run server
# In another terminal:
npm run dev
```

- [ ] **Step 2: State the Provider and safety contract**

```md
`ARK_API_KEY` is optional for browsing the seeded demo. Without it, the app
shows a clear unavailable state and does not send generation requests.
```

- [ ] **Step 3: State the delivery boundary**

```md
All delivery actions are local simulations. This MVP never connects to or
writes to a real advertising platform.
```

- [ ] **Step 4: Mark Task 5 and all subtasks complete**

```md
- [x] Task 5: ...
  - [x] SubTask 5.1: ...
```

### Task 4: Verify and Clean Up

**Files:**
- Modify: only files necessary to correct verification failures

- [ ] **Step 1: Run static checks and tests**

Run: `npm run check:server && npm run test:server && npm run build`
Expected: all commands exit with code 0.

- [ ] **Step 2: Run an API smoke sequence against the local process**

```bash
curl -fsS http://127.0.0.1:8787/health
curl -fsS http://127.0.0.1:8787/api/projects
curl -fsS http://127.0.0.1:8787/api/provider/capabilities
```

Expected: health is `ok`, one seeded project is listed, and the capability response contains no credential.

- [ ] **Step 3: Validate the browser route guide and no-key fallback**

Expected: the dashboard exposes all five route steps, the audit route displays server events, and the no-key message says generation is unavailable without exposing a secret.

- [ ] **Step 4: Remove generated persistence, build output, logs, and temporary files**

Run: `git status --short && git diff --check`
Expected: no generated artifacts are tracked and no whitespace errors are reported.

## Self-Review

- Spec coverage: the demo seeder and guide satisfy the investor-demo path; the server-derived unavailable state satisfies no-Provider degradation; bilingual documentation covers runtime, credentials, and simulation boundaries; the final checks cover build, server tests, API smoke, and browser navigation.
- Security: no actual key is written to source, test output, runtime persistence, browser storage, or documentation examples.
- Scope: the work does not add a real advertising-platform connector or browser-side credential handling.
