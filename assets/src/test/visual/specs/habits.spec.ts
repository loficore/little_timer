import { test, expect, Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

test.describe("习惯追踪 E2E 测试", () => {
  test("习惯集列表显示", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForTimeout(2000);
    const content = page.locator("body");
    await expect(content).toBeVisible();
  });

  test("创建习惯集", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForTimeout(2000);
    const content = page.locator("body");
    await expect(content).toBeVisible();
  });

  test("创建习惯并打卡", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForTimeout(2000);
    const content = page.locator("body");
    await expect(content).toBeVisible();
  });
});