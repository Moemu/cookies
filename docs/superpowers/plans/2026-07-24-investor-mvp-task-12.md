# Investor MVP Task 12 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render project runtime and operational report data from server-owned records after refresh.

**Architecture:** Keep `Project.runtime` as the single source for project-level fields and use current-project operational records for report-specific metrics and recommendations. File repository normalization supplies identifiable defaults to persisted projects created before the runtime field existed.

**Tech Stack:** TypeScript, Node.js HTTP server, file-backed JSON repository, React, Vite, Node test runner.

---

### Task 1: Verify Runtime Persistence

**Files:**
- Modify: `server/operations.test.ts`

- [ ] **Step 1: Add a legacy-project runtime normalization test**

```ts
assert.deepEqual(project.runtime, {
  code: "PRJ",
  product: "",
  stage: "需求梳理",
  progress: 0,
  status: "active",
  owner: "demo-user",
  budget: 0,
  currency: "CNY",
  timezone: "Asia/Shanghai",
});
```

- [ ] **Step 2: Run the focused server test**

Run: `npm run test:server -- server/operations.test.ts`
Expected: PASS with persisted runtime defaults and project-scoped records.

### Task 2: Consume Server-Owned Report Data

**Files:**
- Modify: `server/demo.ts`
- Modify: `src/components/SpecializedPages.tsx`

- [ ] **Step 1: Add persisted report summary fields to the metric seed record**
- [ ] **Step 2: Render the metric summary, scope, and recommendation from `currentProject.operations`**
- [ ] **Step 3: Build the frontend**

Run: `npm run build`
Expected: PASS without fixed report metrics being displayed for every project.

### Task 3: Record Acceptance

**Files:**
- Modify: `.trae/specs/build-investor-mvp/tasks.md`
- Modify: `.trae/specs/build-investor-mvp/checklist.md`

- [ ] **Step 1: Mark Task 12 and its subtasks complete after all checks pass**
- [ ] **Step 2: Record runtime mapping, isolation, and refresh evidence**
