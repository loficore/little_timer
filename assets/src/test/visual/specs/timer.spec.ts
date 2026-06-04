import { test, expect, Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

async function setTimeDisplayStyle(page: Page, style: "classic" | "seven_segment") {
  await page.evaluate((s) => {
    localStorage.setItem("time_display_style", s);
  }, style);
}

test.describe("TimerPage VRT 截图测试", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForTimeout(1500);
  });

  test("计时器整体截图（数码管风格）", async ({ page }) => {
    await setTimeDisplayStyle(page, "seven_segment");
    await page.reload();
    await page.waitForTimeout(1500);
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await expect(page.locator(".my-sidebar")).toBeVisible();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-display-seven-segment.png", {
      maxDiffPixels: 100,
    });
  });

  test("计时器整体截图（经典风格）", async ({ page }) => {
    await setTimeDisplayStyle(page, "classic");
    await page.reload();
    await page.waitForTimeout(1500);
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-display-classic.png", {
      maxDiffPixels: 100,
    });
  });

  test("时钟区域垂直居中（计时器运行中）", async ({ page }) => {
    await setTimeDisplayStyle(page, "seven_segment");
    await page.reload();
    await page.waitForTimeout(1500);

    const startBtn = page.locator('[data-testid="timer-start"]');
    if (await startBtn.isVisible()) {
      await startBtn.click();
      await page.waitForTimeout(2000);
    }

    const glass = page.locator(".my-clock-glass");
    await expect(glass).toBeVisible();
    await expect(glass).toHaveScreenshot("timer-clock-vertical-center.png", {
      maxDiffPixels: 100,
    });
  });

  test("控制按钮等高（习惯选择 + 模式选择）", async ({ page }) => {
    await page.waitForTimeout(1500);
    const habitBtn = page.locator(".my-surface-card").first();
    const modeBtn = page.locator(".dropdown-select-btn").first();

    await expect(habitBtn).toBeVisible();
    await expect(modeBtn).toBeVisible();
    await page.waitForTimeout(500);

    const container = page.locator(".my-clock-glass").first();
    await expect(container).toHaveScreenshot("timer-control-buttons-height.png", {
      maxDiffPixels: 100,
    });
  });

  test("侧边栏和顶栏视觉（分界线减弱后）", async ({ page }) => {
    await page.waitForTimeout(1500);
    await expect(page.locator(".my-sidebar")).toBeVisible();
    await expect(page.locator(".my-topbar")).toBeVisible();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-sidebar-visuals.png", {
      maxDiffPixels: 100,
    });
  });
});