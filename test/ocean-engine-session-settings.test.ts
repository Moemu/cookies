import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

test("巨量会话设置只提交受控输入并仅展示安全元数据", () => {
  const settings = readFileSync(
    new URL("../src/components/OceanEngineSessionSettings.tsx", import.meta.url),
    "utf8",
  );
  const globalSettingsPage = readFileSync(
    new URL("../src/components/ModelSettingsPage.tsx", import.meta.url),
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
  assert.match(settings, /inputRef = useRef<HTMLInputElement>/);
  assert.match(settings, /type="password"/);
  assert.match(settings, /autoComplete="off"/);
  assert.match(settings, /api\.updateOceanEngineSession/);
  assert.match(settings, /inputRef\.current\) inputRef\.current\.value = ''/);
  assert.match(settings, /api\.verifyOceanEngineSession/);
  assert.match(settings, /ocean_engine \/ web_api/);
  assert.doesNotMatch(settings, /useState\([^)]*session/i);
  assert.doesNotMatch(settings, /localStorage|sessionStorage/);
  assert.doesNotMatch(settings, /computer_use|Playwright/);
  assert.match(globalSettingsPage, /OceanEngineSessionSettings/);
  assert.doesNotMatch(settingsPage, /OceanEngineSessionSettings|ocean-engine-session/);
  assert.doesNotMatch(navigation, /巨量会话|ocean-engine-session/);
  const router = readFileSync(
    new URL("../src/lib/router.ts", import.meta.url),
    "utf8",
  );
  assert.match(router, /parts\[0\] === ['"]settings['"]/);
});
