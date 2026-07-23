# Task 6 Frontend Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the frontend project, artifact, and controlled ChangeSet workflow reflect only persisted server state.

**Architecture:** `ProjectContext` loads projects together with artifacts, jobs, and ChangeSets, and exposes asynchronous commands that update local state only after API success. Delivery and approval views use those commands for the selected project instead of creating shadow projects or local records. Media synchronization remains owned by the existing generation flow.

**Tech Stack:** React 19, TypeScript, Node HTTP API, file-backed repository, Node test runner.

---

### Task 1: Unify Client And Context State

**Files:**
- Modify: `src/data/api.ts`
- Modify: `src/context/ProjectContext.tsx`
- Modify: `src/types.ts`

- [ ] Add typed ChangeSet list/create/preflight/approve/execute/rollback client methods to the primary API client.
- [ ] Load ChangeSets with each project and map server timestamps, versions, artifacts, and statuses into project view state.
- [ ] Replace fixed timestamps, generated ChangeSet IDs, and `initialProjects` fallback writes with async server commands.
- [ ] Preserve existing visible state on reload failure and surface the recoverable API error.

### Task 2: Use The Current Project In Delivery Views

**Files:**
- Modify: `src/components/Pages.tsx`
- Modify: `src/components/SpecializedPages.tsx`

- [ ] Persist artifact edits through context commands before success notices.
- [ ] Create and preflight ChangeSets for the selected project using its persisted artifact IDs.
- [ ] Approve, execute, and rollback through context commands, then render the returned server state.
- [ ] Do not edit image/video generation, polling, cancellation, or provider behavior.

### Task 3: Verify Recovery

**Files:**
- Modify: `server/test/task1.test.ts`
- Modify: `.trae/specs/build-investor-mvp/tasks.md`

- [ ] Add a persistence test that reopens the repository after a ChangeSet reaches rollback and confirms artifacts, ChangeSet state, and audit evidence recover.
- [ ] Run server tests, frontend build, API smoke verification, and whitespace checks.
- [ ] Mark Task 6 and all subtasks complete after verification succeeds.
