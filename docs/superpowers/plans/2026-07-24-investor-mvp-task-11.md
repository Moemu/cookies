# Investor MVP Task 11 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist the demo project's operational records on the server and render every operational surface from the current project's API data.

**Architecture:** Store operational records as first-class, project-owned records in the JSON repository and initialize only stable demo records through idempotent seeding. The frontend loads those records alongside each project and maps server `runtime` into the existing `ProjectRecord`, so project cards and cross-page surfaces use one source of truth.

**Tech Stack:** TypeScript, Node.js HTTP server, file-backed JSON repository, React, Vite, Node test runner.

---

### Task 1: Verify Server-Owned Operational Data

**Files:**
- Modify: `server/repository.ts`
- Modify: `server/index.ts`
- Modify: `server/operations.test.ts`

- [ ] **Step 1: Write the failing contract test**

```ts
const records = await request(`/api/projects/${demo.id}/operations`);
assert.equal(records.status, 200);
assert.equal(records.body[0].projectId, demo.id);
assert.deepEqual(await request(`/api/projects/${other.id}/operations`), {
  status: 200,
  body: [],
});
```

- [ ] **Step 2: Run the focused test**

Run: `npm run test:server -- server/operations.test.ts`
Expected: PASS after the operational-record endpoint returns only records owned by the requested project.

- [ ] **Step 3: Implement deterministic persisted lookup**

```ts
async listOperationalRecords(projectId?: string): Promise<OperationalRecord[]> {
  return this.store.operationalRecords
    .filter((record) => !projectId || record.projectId === projectId)
    .sort((left, right) => right.occurredAt.localeCompare(left.occurredAt) || left.id.localeCompare(right.id));
}
```

- [ ] **Step 4: Re-run the focused test**

Run: `npm run test:server -- server/operations.test.ts`
Expected: PASS.

### Task 2: Map Runtime and Remove Client Business Mocks

**Files:**
- Modify: `src/data/api.ts`
- Modify: `src/context/ProjectContext.tsx`
- Delete: `src/data/mock.ts`
- Delete: `src/data/projects.ts`

- [ ] **Step 1: Define the API runtime contract**

```ts
export type ApiProject = {
  id: string
  name: string
  brand: string
  objective: string
  runtime: {
    code: string
    product: string
    stage: string
    progress: number
    status: 'active' | 'completed'
    owner: string
    budget: number
    currency: 'CNY'
    timezone: 'Asia/Shanghai'
  }
}
```

- [ ] **Step 2: Map server runtime in `toProjectRecord`**

```ts
code: project.runtime.code,
product: project.runtime.product,
stage: project.runtime.stage,
progress: project.runtime.progress,
status: project.runtime.status === 'completed' ? '已完成' : '进行中',
budget: project.runtime.budget,
currency: project.runtime.currency,
timezone: project.runtime.timezone,
```

- [ ] **Step 3: Remove obsolete client business records**

Delete `src/data/mock.ts` and `src/data/projects.ts` after confirming no source files import them. Page state must show the existing recoverable API error rather than restore deleted mock records.

- [ ] **Step 4: Build the frontend**

Run: `npm run build`
Expected: TypeScript completes without imports to the deleted mock modules.

### Task 3: Complete Acceptance Evidence

**Files:**
- Modify: `.trae/specs/build-investor-mvp/tasks.md`
- Modify: `.trae/specs/build-investor-mvp/checklist.md`
- Test: `server/operations.test.ts`

- [ ] **Step 1: Run server static checks**

Run: `npm run check:server`
Expected: PASS.

- [ ] **Step 2: Run all server tests**

Run: `npm run test:server`
Expected: PASS.

- [ ] **Step 3: Run persistence smoke verification**

Run: `npx tsx --eval "import { FileRepository } from './server/repository.ts'; import { seedDemoProject } from './server/demo.ts'; const r = await FileRepository.open('/tmp/cookies-task-11.json'); const p = await seedDemoProject(r); console.log((await r.listOperationalRecords(p.id)).length);"`
Expected: a stable nonzero count after repeated runs.

- [ ] **Step 4: Mark completed requirements**

Mark Task 11 and its five subtasks complete only after the checks above pass, then mark the five matching operational-data items in the checklist complete.
