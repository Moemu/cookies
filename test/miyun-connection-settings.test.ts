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
  assert.doesNotMatch(settings, /useState\([^)]*session/i);
  assert.match(page, /href="\/settings"/);
  assert.match(page, /void load\(true\)/);
  assert.doesNotMatch(page, /setSession/);
});
