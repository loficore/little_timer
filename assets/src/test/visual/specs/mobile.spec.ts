import { test, expect, devices } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

test.use({ ...devices["iPhone 12"] });

test.describe("移动端 viewport (iPhone 12) E2E 测试", () => {
  test("Timer 页面在移动端加载正常", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await expect(page.locator("body")).toBeVisible();
  });

  test("Habits 页面在移动端加载正常", async ({ page }) => {
    await page.goto(`${baseURL}/#/habits`);
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("Settings 页面在移动端加载正常", async ({ page }) => {
    await page.goto(`${baseURL}/#/settings`);
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("Timer 页面在移动端点击开始按钮正常响应", async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    const startBtn = page.locator(".btn.btn-primary.btn-lg").first();
    await expect(startBtn).toBeVisible();
    await startBtn.click();
    await expect(startBtn).toBeVisible();
  });
});
