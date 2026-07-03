import { test, expect, Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

test.describe("页面导航 E2E 测试", () => {
  test("页面首次加载", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await expect(page.locator("body")).toBeVisible();
  });

  test("导航到习惯页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await page.evaluate(() => {
      window.location.hash = "#/habits";
    });
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("导航到设置页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await page.evaluate(() => {
      window.location.hash = "#/settings";
    });
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("导航到统计页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await page.evaluate(() => {
      window.location.hash = "#/stats";
    });
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("从习惯页导航回计时器", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForLoadState("networkidle");
    await page.evaluate(() => {
      window.location.hash = "";
    });
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });
});
