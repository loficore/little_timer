import { test, expect, Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

test.describe("页面导航 E2E 测试", () => {
  test("页面首次加载", async ({ page }) => {
    await page.goto(baseURL);
    await expect(page.locator("html")).toBeVisible();
  });

  test("导航到习惯页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForTimeout(1000);
    await page.evaluate(() => {
      window.location.hash = "#/habits";
    });
    await page.waitForTimeout(500);
  });

  test("导航到设置页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForTimeout(1000);
    await page.evaluate(() => {
      window.location.hash = "#/settings";
    });
    await page.waitForTimeout(500);
  });

  test("导航到统计页面", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForTimeout(1000);
    await page.evaluate(() => {
      window.location.hash = "#/stats";
    });
    await page.waitForTimeout(500);
  });

  test("导航到计时器页面", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForTimeout(1000);
    await page.evaluate(() => {
      window.location.hash = "";
    });
    await page.waitForTimeout(500);
  });
});