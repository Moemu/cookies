# Task 14.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the post-launch analysis consume only the current Project's persisted operational records, with recoverable empty states and server-backed refresh.

**Architecture:** Persist per-ad review input alongside existing metric, evidence, and delivery-action operational records. The React page derives all conclusions, reasons, recommendations, filtering, drill-down, and report content from `currentProject.operations`, which ProjectContext loads from the per-project API.

**Tech Stack:** TypeScript, React, Node.js test runner, file-backed MVP repository.

---

### Task 1: Persist Review Inputs

**Files:**
- Modify: `server/domain.ts`
- Modify: `server/demo.ts`
- Test: `server/operations.test.ts`

- [x] **Step 1: Write a failing server contract assertion**

```ts
assert.equal(demoRecords.body.some((record: { kind: string }) => record.kind === "performance_ad"), true);
```

- [x] **Step 2: Run the focused test**

Run: `npx tsx --test server/operations.test.ts`
Expected: FAIL because the operation record kind does not exist.

- [x] **Step 3: Add the record kind and project-scoped seed rows**

```ts
record("AD-2607-031", projectId, "performance_ad", "精度证据·研发负责人", "持续放量", "2026-07-22T08:30:00.000Z", {
  platform: "巨量引擎",
  format: "视频",
  spend: 28640,
  impressions: 682400,
  ctr: 4.18,
  cpa: 54.2,
})
```

- [x] **Step 4: Run the focused test**

Run: `npx tsx --test server/operations.test.ts`
Expected: PASS.

### Task 2: Replace Client Mock Analysis

**Files:**
- Modify: `src/components/CoreFlowPages.tsx`

- [x] **Step 1: Replace the fixed `adRows` source with typed selectors over `currentProject.operations`**

```ts
const rows = useMemo(() => currentProject.operations
  .filter((record) => record.kind === "performance_ad")
  .map(toPerformanceRow), [currentProject.operations])
```

- [x] **Step 2: Derive trends, reasons, recommendations, and report content from project operations**

```ts
const metrics = currentProject.operations.filter((record) => record.kind === "metric")
const reasons = currentProject.operations.filter((record) => record.kind === "evidence")
const actions = currentProject.operations.filter((record) => record.kind === "delivery_action")
```

- [x] **Step 3: Make Refresh call `reloadProjects`, and add clear-filter/retry empty states**

```ts
await reloadProjects()
setNotice("已从服务端刷新当前 Project 的运营记录。")
```

- [x] **Step 4: Run the frontend build**

Run: `npm run build`
Expected: PASS.

### Task 3: Verify Delivery

**Files:**
- Modify: `.trae/specs/build-investor-mvp/tasks.md`

- [x] **Step 1: Mark SubTask 14.1 complete after checks pass**

- [x] **Step 2: Run repository checks**

Run: `npm run test:server && npm run check:server && npm run build && git diff --check`
Expected: all commands exit with code 0.
