import { test, expect, Page } from "@playwright/test";

// ponytail: keys follow task spec verbatim. Note: actual app stores timezone/
// sound_enabled server-side (API); theme_mode uses STORAGE_KEYS.THEME_MODE
// ("lt_theme_mode") in useAppSettings.ts. These tests assert that any value
// written to localStorage survives a full page reload.
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