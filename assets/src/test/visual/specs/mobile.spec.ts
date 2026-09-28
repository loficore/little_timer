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
    // 未选习惯时点击「开始」会弹出习惯选择器而不是启动计时，测试前先选一个
    const pickerBtn = page.locator('[data-testid="timer-habit-picker"]');
    await pickerBtn.click();
    await page.waitForSelector(".my-surface-modal", { state: "visible", timeout: 5000 });
    await page.locator('[data-testid^="habit-option-"]').last().click();
    await page.waitForSelector(".my-surface-modal", { state: "hidden", timeout: 5000 });

    const startBtn = page.locator('[data-testid="timer-start"]');
    await expect(startBtn).toBeVisible();
    await startBtn.click();
    // 立即暂停并重置，避免污染后端全局计时器影响后续用例
    const pauseBtn = page.locator('[data-testid="timer-pause"]');
    await expect(pauseBtn).toBeVisible({ timeout: 5000 });
    await pauseBtn.click();
    await page.locator('[data-testid="timer-reset"]').click();
    await expect(startBtn).toBeVisible({ timeout: 5000 });
  });
});
