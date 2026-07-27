import assert from "node:assert/strict";
import test from "node:test";
import { agencyWorkbenchSample, api } from "../src/data/api.ts";

test("agency workbench sample is opt-in and can be scoped to selected Projects", async () => {
  const defaultWorkbench = await api.listAgencyWorkbench();
  assert.equal(defaultWorkbench.projects.length, 0);
  assert.equal(defaultWorkbench.qualityCheckRuns.length, 0);

  const scoped = await api.listAgencyWorkbench({ projectIds: ["project-nova-home-launch"] });
  assert.deepEqual(scoped.projects.map((project) => project.id), ["project-nova-home-launch"]);
  assert.equal(scoped.qualityCheckRuns.every((run) => run.projectId === "project-nova-home-launch"), true);
  assert.equal(scoped.materialConfirmations.every((item) => item.projectId === "project-nova-home-launch"), true);
  assert.equal(scoped.assetVersionPointers.every((pointer) => pointer.projectId === "project-nova-home-launch"), true);
  assert.equal(scoped.adAccountBindings.every((binding) => binding.projectIds.every((projectId) => projectId === "project-nova-home-launch")), true);

  const portfolio = await api.listAgencyWorkbench({ includePortfolioSample: true });
  assert.equal(portfolio.projects.length, agencyWorkbenchSample.projects.length);
  assert.equal(portfolio.qualityCheckRuns.length, agencyWorkbenchSample.qualityCheckRuns.length);
});
