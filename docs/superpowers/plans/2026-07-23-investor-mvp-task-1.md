# Investor MVP Task 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a locally runnable, durable HTTP API for the five MVP domain resources, including validated state transitions and tests.

**Architecture:** A small TypeScript service runs on Node's built-in `http` server, so the existing Vite app gains no runtime framework dependency. Domain schemas and transition rules live in one module; a JSON-file repository atomically persists state and audit events; the HTTP router validates request bodies and maps all known failures to one public error envelope.

**Tech Stack:** Node.js `http`, TypeScript, `tsx`, Node.js `node:test`, JSON file persistence.

---

### Task 1: Server Runtime and Domain

**Files:**
- Create: `server/domain.ts`
- Create: `server/errors.ts`
- Create: `server/repository.ts`
- Create: `server/tsconfig.json`
- Modify: `package.json`

- [ ] **Step 1: Add server commands and test runner**

```json
{
  "scripts": {
    "server": "tsx server/index.ts",
    "test:server": "tsx --test server/**/*.test.ts",
    "check:server": "tsc --noEmit -p server/tsconfig.json"
  }
}
```

- [ ] **Step 2: Define immutable resource schemas and legal transitions**

```ts
export type GenerationJobStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled";

const generationJobTransitions = {
  queued: ["running", "cancelled"],
  running: ["succeeded", "failed", "cancelled"],
  succeeded: [],
  failed: ["queued"],
  cancelled: ["queued"],
} as const;

export function assertTransition(
  current: GenerationJobStatus,
  next: GenerationJobStatus,
): void {
  if (!generationJobTransitions[current].includes(next)) {
    throw new DomainError("INVALID_STATE_TRANSITION", "...");
  }
}
```

- [ ] **Step 3: Persist one store document with atomic writes**

```ts
await mkdir(dirname(this.filePath), { recursive: true });
await writeFile(`${this.filePath}.tmp`, JSON.stringify(store, null, 2));
await rename(`${this.filePath}.tmp`, this.filePath);
```

- [ ] **Step 4: Run the server type check**

Run: `npm run check:server`
Expected: exit code 0.

### Task 2: HTTP API and Auditing

**Files:**
- Create: `server/index.ts`
- Create: `server/http.ts`
- Modify: `.gitignore`

- [ ] **Step 1: Implement health and collection/item routes**

```ts
GET  /health
GET  /api/projects
POST /api/projects
GET  /api/projects/:id
PATCH /api/projects/:id
GET  /api/artifacts?projectId=:projectId
POST /api/artifacts
GET  /api/generation-jobs?projectId=:projectId
POST /api/generation-jobs
PATCH /api/generation-jobs/:id
GET  /api/change-sets?projectId=:projectId
POST /api/change-sets
PATCH /api/change-sets/:id
GET  /api/audit-events?projectId=:projectId
```

- [ ] **Step 2: Return one safe public error format**

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "projectId is required",
    "details": [{ "field": "projectId", "message": "Required" }]
  }
}
```

- [ ] **Step 3: Append audit events for every create, update, and state transition**

```ts
repository.appendAudit({
  projectId: entity.projectId,
  actor: input.actor ?? "demo-user",
  action: "generation_job.status_changed",
  entityType: "generation_job",
  entityId: entity.id,
  metadata: { from: currentStatus, to: nextStatus },
});
```

- [ ] **Step 4: Ignore runtime persistence files**

```gitignore
data/
```

### Task 3: Back-End Tests and Verification

**Files:**
- Create: `server/server.test.ts`
- Modify: `package.json`

- [ ] **Step 1: Write state-transition tests**

```ts
assert.throws(
  () => assertGenerationJobTransition("succeeded", "running"),
  (error: unknown) => isDomainError(error, "INVALID_STATE_TRANSITION"),
);
```

- [ ] **Step 2: Test repository restart recovery and audit persistence**

```ts
const first = await FileRepository.open(filePath);
const project = await first.createProject({ name: "Demo", brand: "Cookies", objective: "Grow" });
const second = await FileRepository.open(filePath);
assert.equal((await second.getProject(project.id))?.name, "Demo");
```

- [ ] **Step 3: Test health, validation, not-found and invalid-transition error boundaries**

```ts
const response = await fetch(`${baseUrl}/api/generation-jobs/${job.id}`, {
  method: "PATCH",
  body: JSON.stringify({ status: "running" }),
});
assert.deepEqual(await response.json(), {
  error: {
    code: "INVALID_STATE_TRANSITION",
    message: "Cannot transition generation job from succeeded to running",
  },
});
```

- [ ] **Step 4: Run tests and smoke checks**

Run: `npm run check:server && npm run test:server && npm run build`
Expected: all commands exit with code 0.

- [ ] **Step 5: Commit**

```bash
git add package.json package-lock.json .gitignore server docs/superpowers/plans
git commit -m "feat: add persistent MVP backend foundation"
```

## Self-Review

- Spec coverage: Task 1.1 maps to `server/domain.ts`; Task 1.2 maps to `server/repository.ts`, `server/http.ts`, and `server/index.ts`; Task 1.3 maps to `server/server.test.ts`.
- Deliberate boundary: no Provider configuration, credentials, model invocations, frontend client, or live advertising execution are added because they are Tasks 2–4.
- API contract: all resource writes accept `actor` only as demo audit attribution; authentication and authorization remain future contracts.
