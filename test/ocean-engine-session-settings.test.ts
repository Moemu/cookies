import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

test("巨量 Connector 由 Project 持有且只提交受控输入", () => {
  const settings = readFileSync(
    new URL("../src/components/OceanEngineSessionSettings.tsx", import.meta.url),
    "utf8",
  );
  const globalSettingsPage = readFileSync(
    new URL("../src/components/ModelSettingsPage.tsx", import.meta.url),
    "utf8",
  );
  const deliveryPages = readFileSync(
    new URL("../src/components/Pages.tsx", import.meta.url),
    "utf8",
  );
  const settingsPage = readFileSync(
    new URL("../src/components/insight/settings/SettingsPage.tsx", import.meta.url),
    "utf8",
  );
  const navigation = readFileSync(
    new URL("../src/data/navigation.ts", import.meta.url),
    "utf8",
  );
  assert.match(settings, /externalIDRef = useRef<HTMLInputElement>/);
  assert.match(settings, /sessionInputRef = useRef<HTMLInputElement>/);
  assert.match(settings, /type="password"/);
  assert.match(settings, /autoComplete="off"/);
  assert.match(settings, /\{ projectId \}: \{ projectId: string \}/);
  assert.match(settings, /api\.registerProjectConnectorAccount\(projectId/);
  assert.match(settings, /api\.updateProjectConnectorAccountSession\(projectId/);
  assert.match(settings, /sessionInputRef\.current\) sessionInputRef\.current\.value = ''/);
  assert.match(settings, /api\.verifyProjectConnectorAccount\(projectId/);
  assert.match(settings, /api\.syncProjectConnectorAccount\(projectId/);
  assert.match(settings, /账号只归属当前 Project/);
  assert.doesNotMatch(settings, /useProject/);
  assert.doesNotMatch(settings, /useState\([^)]*session/i);
  assert.doesNotMatch(settings, /localStorage|sessionStorage/);
  assert.doesNotMatch(settings, /computer_use|Playwright/);
  assert.doesNotMatch(globalSettingsPage, /OceanEngineSessionSettings/);
  assert.match(deliveryPages, /<OceanEngineSessionSettings projectId=\{currentProject\.id\}\/>/);
  assert.doesNotMatch(settingsPage, /OceanEngineSessionSettings|ocean-engine-session/);
  assert.doesNotMatch(navigation, /巨量会话|ocean-engine-session/);
  const router = readFileSync(
    new URL("../src/lib/router.ts", import.meta.url),
    "utf8",
  );
  assert.match(router, /parts\[0\] === ['"]settings['"]/);
});
