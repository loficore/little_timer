import { test, expect, type Page } from "@playwright/test";
import http from "http";
import { expectButtonContrast } from "../helpers/contrast";

const baseURL = "http://127.0.0.1:5173";

type PageId = "timer" | "habits" | "schedule" | "settings" | "aigtd";

const CASES: Array<[name: string, page: PageId, testid: string, min: number]> = [
  ["timer 开始", "timer", "timer-start", 4.5],
  ["timer 重置", "timer", "timer-reset", 4.5],
  ["习惯集添加", "habits", "create-habit-set", 4.5],
  ["调度运行", "schedule", "run-scheduler", 4.5],
  ["壁纸入口", "settings", "wallpaper-entry", 3.0],
  ["AIGTD 提交", "aigtd", "aigtd-submit", 3.0],
];

interface ServerSettings {
  basic: Record<string, unknown>;
  clock_defaults?: unknown;
}

/** worker 级 node http 直连后端，不依赖浏览器 fixture（测试超时后仍可还原） */
function api(method: "GET" | "POST", body?: unknown): Promise<ServerSettings> {
  return new Promise((resolve, reject) => {
    const payload = body === undefined ? undefined : Buffer.from(JSON.stringify(body));
    const req = http.request(
      {
        hostname: "127.0.0.1",
        port: 8080,
        path: "/api/settings",
        method,
        headers: {
          "Content-Type": "application/json",
          ...(method === "POST" ? { "If-Match": '"0"', "Content-Length": payload!.byteLength } : {}),
        },
      },
      (res) => {
        let text = "";
        res.on("data", (c) => (text += c));
        res.on("end", () => {
          if (res.statusCode !== 200) {
            reject(new Error(`${method} /api/settings → ${res.statusCode}: ${text.slice(0, 200)}`));
            return;
          }
          try {
            resolve(JSON.parse(text) as ServerSettings);
          } catch (e) {
            reject(e as Error);
          }
        });
      },
    );
    req.on("error", reject);
    if (payload) req.write(payload);
    req.end();
  });
}

async function applyTheme(baseline: ServerSettings, theme: string): Promise<void> {
  await api("POST", { ...baseline, basic: { ...baseline.basic, theme_mode: theme } });
}

async function waitForTheme(page: Page, theme: "dark" | "light"): Promise<void> {
  await page.waitForLoadState("load");
  await expect
    .poll(
      async () =>
        page.evaluate(() => document.documentElement.classList.contains("light-mode")),
      { message: `等待主题生效: ${theme}`, timeout: 15000 },
    )
    .toBe(theme === "light");
}

async function gotoPage(page: Page, id: PageId): Promise<void> {
  await page.locator(`[data-testid="nav-${id}"]:visible`).first().click();
  if (id === "settings") {
    await page.locator('[data-testid="tab-basic"]:visible').first().click();
  }
  if (id === "aigtd") {
    // 空输入时提交箭头 disabled（opacity 干扰采样），填入文本使其进入可用态
    await page.locator('[data-testid="capture-input"]').fill("对比度采样");
  }
  await page.waitForTimeout(400);
}

test.describe("按钮 WCAG AA 对比度像素断言", () => {
  // 共享后端 theme_mode：与 timer.spec 一样强制单 worker，避免并行互踩
  test.describe.configure({ timeout: 180_000, mode: "serial" });
  for (const theme of ["dark", "light"] as const) {
    test(`按钮对比度 AA (${theme})`, async ({ page }) => {
      const baseline = await api("GET");
      try {
        await applyTheme(baseline, theme);
        await page.goto(baseURL);
        await waitForTheme(page, theme);
        for (const [name, pg, id, min] of CASES) {
          await gotoPage(page, pg);
          await expectButtonContrast(
            page.locator(`[data-testid="${id}"]:visible`).first(),
            min,
            `${name} (${theme})`,
          );
        }
      } finally {
        // 还原为读取时的主题（globalSetup 基线为 light），避免污染其余 VRT 用例
        await applyTheme(baseline, String(baseline.basic.theme_mode ?? "light")).catch(() => {});
      }
    });
  }
});
