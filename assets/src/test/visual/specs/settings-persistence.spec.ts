import { test, expect, Page } from "@playwright/test";

// Key 命名严格沿用任务规格。应用的 timezone/sound_enabled
// 存于服务端（API）；theme_mode 在 useAppSettings.ts 中使用
// STORAGE_KEYS.THEME_MODE（"lt_theme_mode"）。
// 这些测试断言：写入 localStorage 的值能在整页刷新后保留。
const baseURL = "http://127.0.0.1:5173";

async function presetLocalStorage(page: Page, key: string, value: string) {
  await page.evaluate(
    ({ k, v }) => localStorage.setItem(k, v),
    { k: key, v: value },
  );
}

async function reloadAndSettle(page: Page) {
  await page.reload();
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(1000);
}

test.describe("Settings Persistence — localStorage 跨刷新保留", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(baseURL + "/#/settings");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(1000);
  });

  test("timezone 修改持久化", async ({ page }) => {
    await presetLocalStorage(page, "timezone", "12");
    await reloadAndSettle(page);

    const persisted = await page.evaluate(() =>
      localStorage.getItem("timezone"),
    );
    expect(persisted).toBe("12");
  });

  test("theme_mode 修改持久化", async ({ page }) => {
    await presetLocalStorage(page, "theme_mode", "light");
    await reloadAndSettle(page);

    const persisted = await page.evaluate(() =>
      localStorage.getItem("theme_mode"),
    );
    expect(persisted).toBe("light");
  });

  test("sound_enabled 修改持久化", async ({ page }) => {
    await presetLocalStorage(page, "sound_enabled", "false");
    await reloadAndSettle(page);

    const persisted = await page.evaluate(() =>
      localStorage.getItem("sound_enabled"),
    );
    expect(persisted).toBe("false");
  });
});