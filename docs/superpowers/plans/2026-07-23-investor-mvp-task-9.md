# Investor MVP Task 9 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the persisted investor walkthrough project complete, repeatable, and safe to initialize alongside existing user projects.

**Architecture:** Extract a startup repository initializer that opens the file store and runs an idempotent demo reconciler. The reconciler identifies only the canonical preset project, appends missing ready artifacts, a currently approvable ChangeSet, and seed audit evidence without changing user-owned records. API tests exercise the seeded resources through approval, simulated execution, audit retrieval, and a restart.

**Tech Stack:** TypeScript, Node.js `node:test`, file-backed JSON repository, native HTTP API.

---

### Task 1: Reconcile preset data during startup

**Files:**
- Modify: `server/demo.ts`
- Modify: `server/index.ts`

- [ ] **Step 1: Define the canonical preset identity and required resources**

```ts
const demoProjectIdentity = {
  name: DEMO_PROJECT_NAME,
  brand: "白域精工",
  objective: "向采购与研发负责人展示精度证据，获取高质量销售线索。",
};
```

- [ ] **Step 2: Reuse the canonical preset or create it without touching other projects**

```ts
const project = (await repository.listProjects()).find(matchesDemoProject)
  ?? await repository.createProject({ ...demoProjectIdentity, actor: "demo-seeder" });
```

- [ ] **Step 3: Append only missing ready Brief, ready creative, and approvable ChangeSet**

```ts
const brief = artifacts.find((artifact) => artifact.kind === "brief" && artifact.status === "ready")
  ?? await repository.createArtifact({ projectId: project.id, kind: "brief", status: "ready", content, actor });
```

- [ ] **Step 4: Add immutable seed verification evidence only when the preset has no audit records**

```ts
if ((await repository.listAuditEvents(project.id)).length === 0) {
  await repository.recordAuditEvent(project.id, "demo.seed_verified", "project", project.id, actor);
}
```

- [ ] **Step 5: Route application startup through a testable initializer**

```ts
export async function openSeededRepository(filePath: string): Promise<FileRepository> {
  const repository = await FileRepository.open(filePath);
  await seedDemoProject(repository);
  return repository;
}
```

### Task 2: Cover startup API smoke and persistence safety

**Files:**
- Modify: `server/test/task1.test.ts`

- [ ] **Step 1: Write a test fixture containing an existing user project and incomplete legacy preset project**

```ts
const userProject = await repository.createProject({
  name: "用户保存的项目",
  brand: "用户品牌",
  objective: "不得被预置初始化覆盖",
});
const legacyDemo = await repository.createProject({ ...demoProjectIdentity });
```

- [ ] **Step 2: Start through the initializer and assert the canonical project was repaired, not replaced**

```ts
const seeded = await openSeededRepository(filePath);
assert.equal((await seeded.getProject(userProject.id))?.name, "用户保存的项目");
assert.equal((await seeded.getProject(legacyDemo.id))?.id, legacyDemo.id);
```

- [ ] **Step 3: Smoke the preset API flow**

```ts
const approved = await app.request("POST", `/api/change-sets/${changeSet.id}/approve`, {
  actor: "demo-smoke",
  role: "demo-approver",
});
assert.equal(approved.body.status, "approved");
assert.equal((await app.request("POST", `/api/change-sets/${changeSet.id}/execute`)).body.status, "executed");
```

- [ ] **Step 4: Reopen the store and confirm user data and audit evidence recover**

```ts
const recovered = await openSeededRepository(filePath);
assert.equal((await recovered.getProject(userProject.id))?.brand, "用户品牌");
assert.ok((await recovered.listAuditEvents(legacyDemo.id)).length > 0);
```

### Task 3: Validate and close the task

**Files:**
- Modify: `.trae/specs/build-investor-mvp/tasks.md`

- [ ] **Step 1: Run focused test**

Run: `npm run test:server`
Expected: all Node test cases pass, including the seeded startup smoke flow.

- [ ] **Step 2: Run static checks and frontend build**

Run: `npm run check:server && npm run build`
Expected: TypeScript checks and Vite production build complete without errors.

- [ ] **Step 3: Mark Task 9 and each subtask complete**

```markdown
- [x] Task 9: 修复既有持久化存储下预置路演项目的可复演性。
  - [x] SubTask 9.1: ...
```
