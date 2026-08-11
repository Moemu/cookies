import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

test("米云会话仅在系统设置中经不受控输入提交，分析页不再保存 Cookie 表单", () => {
  const settings = readFileSync(
    new URL("../src/components/MiyunConnectionSettings.tsx", import.meta.url),
    "utf8",
  );
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(settings, /https:\/\/console\.youshu\.youcloud\.com/);
  assert.match(settings, /sessionInputRef = useRef<HTMLInputElement>/);
  assert.match(settings, /api\.updateMiyunConnection/);
  assert.match(settings, /sessionInputRef\.current\.value = ""/);
  assert.match(settings, /api\.verifyMiyunConnection/);
  assert.match(settings, /function verificationNotice/);
  assert.match(settings, /last_error_kind === "rate_limited"/);
  assert.match(settings, /error\.code === "VERSION_CONFLICT"/);
  assert.doesNotMatch(settings, /error\.status === 409\) return "数据已更新/);
  assert.match(settings, /16356/);
  assert.match(settings, /如果没有 Cookie 这一项就换一个请求/);
  assert.match(settings, /document\.addEventListener\("visibilitychange", refreshOnReturn\)/);
  assert.match(settings, /window\.addEventListener\("focus", refreshOnReturn\)/);
  assert.match(settings, /miyun-connection-metadata/);
  assert.match(settings, /name="miyun-session"/);
  assert.doesNotMatch(settings, /useState\([^)]*session/i);
  assert.match(page, /href="\/settings"/);
  assert.match(page, /void load\(\);/);
  assert.doesNotMatch(page, /api\.verifyMiyunConnection/);
  assert.doesNotMatch(page, /setSession/);
});

test("local launchers enable stable Miyun session encryption", () => {
  const development = readFileSync(
    new URL("../scripts/dev.ps1", import.meta.url),
    "utf8",
  );
  const acceptance = readFileSync(
    new URL("../scripts/local-acceptance-common.ps1", import.meta.url),
    "utf8",
  );
  const requiredSettings = [
    "COOKIES_MIYUN_ENABLED",
    "COOKIES_MIYUN_ENDPOINT",
    "COOKIES_MIYUN_MASTER_KEY",
    "COOKIES_MIYUN_MASTER_KEY_VERSION",
    "COOKIES_MIYUN_DOWNLOAD_ALLOWED_HOSTS",
  ];
  for (const setting of requiredSettings) {
    assert.match(development, new RegExp(setting));
    assert.match(acceptance, new RegExp(setting));
  }
  assert.match(development, /GetEnvironmentVariable\(\$setting\.Key, "Process"\)/);
  assert.match(development, /insights\.read,insights\.write,insights\.confirm/);
  assert.match(acceptance, /insights\.read,insights\.write,insights\.confirm/);
  assert.match(development, /creative-static-ag-v2\.umcdn\.cn/);
  assert.match(acceptance, /creative-static-ag-v2\.umcdn\.cn/);
});
