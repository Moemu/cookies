import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";

test("agency workbench does not invent portfolio data without selected Projects", async () => {
  const defaultWorkbench = await api.listAgencyWorkbench();
  assert.equal(defaultWorkbench.projects.length, 0);
  assert.equal(defaultWorkbench.qualityCheckRuns.length, 0);
  assert.equal(defaultWorkbench.materialConfirmations.length, 0);
  assert.equal(defaultWorkbench.assetVersionPointers.length, 0);
  assert.equal(defaultWorkbench.adAccountBindings.length, 0);
});
