import { test, expect, Page } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";

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
    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    const viewport = page.viewportSize();
    if (viewport && viewport.width >= 1024) {
      await expect(page.locator(".my-sidebar")).toBeVisible();
    }
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-display-seven-segment.png", {
      maxDiffPixels: 100,
    });
  });

  test("计时器整体截图（经典风格）", async ({ page }) => {
    await setTimeDisplayStyle(page, "classic");
    await page.reload();
    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-display-classic.png", {
      maxDiffPixels: 100,
    });
  });

  test("时钟区域垂直居中（计时器运行中）", async ({ page }) => {
    await setTimeDisplayStyle(page, "seven_segment");
    await page.reload();
    await page.waitForLoadState("networkidle");

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
    const targetBtn = page.locator('.my-surface-card').first();
    const modeBtn = page.locator('.dropdown-select-btn').first();

    await expect(targetBtn).toBeVisible();
    await expect(modeBtn).toBeVisible();
    await page.waitForTimeout(500);

    const container = page.locator(".my-clock-glass").first();
    await expect(container).toHaveScreenshot("timer-control-buttons-height.png", {
      maxDiffPixels: 100,
    });
  });

  test("侧边栏和顶栏视觉（分界线减弱后）", async ({ page }) => {
    const viewport = page.viewportSize();
    test.skip(viewport && viewport.width < 1024, "移动端无侧边栏");

    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-sidebar")).toBeVisible();
    await expect(page.locator(".my-topbar")).toBeVisible();
    await expect(page).toHaveScreenshot("timer-sidebar-visuals.png", {
      maxDiffPixels: 100,
    });
  });

  test("主题切换后计时器外观（light 主题）", async ({ page }) => {
    await page.evaluate(() => {
      document.documentElement.classList.add("light-mode");
    });
    await page.reload();
    await page.waitForLoadState("networkidle");
    await expect(page.locator(".my-clock-glass")).toBeVisible();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-theme-light.png", {
      maxDiffPixels: 100,
    });
  });

  test("倒计时和秒表模式切换视觉", async ({ page }) => {
    await page.reload();
    await page.waitForLoadState("networkidle");

    await page.locator('.dropdown-select-btn').click();
    await page.waitForLoadState("networkidle");
    await page.locator('.my-surface-modal button').nth(1).click();
    // Wait for display state transition
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-stopwatch-mode.png", { maxDiffPixels: 100 });

    // Re-open dropdown (text changed to "秒表")
    await page.locator('.dropdown-select-btn').first().click();
    await page.waitForLoadState("networkidle");
    await page.locator('.my-surface-modal button').nth(0).click();
    // Wait for display state transition
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-countdown-mode.png", { maxDiffPixels: 100 });
  });

  test("控制按钮等高（stopwatch 运行中）", async ({ page }) => {
    const startBtn = page.locator('[data-testid="timer-start"]');
    if (await startBtn.isVisible()) {
      await startBtn.click();
      await page.waitForTimeout(1500);
    }
    const container = page.locator(".my-clock-glass").filter({ hasText: "" });
    await expect(container).toHaveScreenshot("timer-stopwatch-running.png", {
      maxDiffPixels: 100,
    });
  });
});

test.describe("Timer 用户旅程 E2E", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(baseURL);
    await page.waitForTimeout(1500);
  });

  test("stopwatch 完整旅程: start → pause → resume → reset", async ({ page }) => {
    const timerPage = new TimerPage(page);

    // 1. Navigate and select stopwatch mode
    await timerPage.goto();

    // 2. Select habit
    await timerPage.selectHabit();

    // 3. Click start - verify timer is running
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    // 3. Wait 1s - verify display ticks
    const displayBefore = await timerPage.getTimerDisplayText();
    await page.waitForTimeout(1000);

    const displayAfter = await timerPage.getTimerDisplayText();
    expect(displayAfter).not.toBe(displayBefore);

    // 4. Click pause - verify paused (neither running nor fully stopped)
    await timerPage.clickPause();
    expect(await timerPage.isTimerStopped()).toBe(false);
    expect(await timerPage.isTimerRunning()).toBe(false);

    // 5. Resume via clickResume - verify running again
    await timerPage.clickResume();
    expect(await timerPage.isTimerRunning()).toBe(true);

    // 6. Click reset - verify back to initial state (start button visible)
    await timerPage.clickReset();
    expect(await timerPage.isTimerStopped()).toBe(true);
    expect(await timerPage.isTimerRunning()).toBe(false);
  });

  test("countdown 流程: start → finish → 验证状态", async ({ page }) => {
    const timerPage = new TimerPage(page);

    await timerPage.goto();
    await timerPage.selectMode("countdown");
    await timerPage.setWorkDuration(5);

    await timerPage.selectHabit();
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    await page.waitForTimeout(6000);

    await timerPage.waitForTimerFinish(10000);

    await timerPage.clickFinish();
    expect(await timerPage.isTimerStopped()).toBe(true);
    expect(await timerPage.isTimerRunning()).toBe(false);
  });

  test("countdown 完整旅程: start pause resume reset", async ({ page }) => {
    const timerPage = new TimerPage(page);

    await timerPage.goto();
    await timerPage.selectMode("countdown");
    await timerPage.setWorkDuration(5);

    await timerPage.selectHabit();
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    await timerPage.clickPause();
    expect(await timerPage.isTimerStopped()).toBe(false);
    expect(await timerPage.isTimerRunning()).toBe(false);

    await timerPage.clickResume();
    expect(await timerPage.isTimerRunning()).toBe(true);

    await timerPage.waitForTimerFinish(10000);

    await timerPage.clickFinish();
    expect(await timerPage.isTimerStopped()).toBe(true);
    expect(await timerPage.isTimerRunning()).toBe(false);
  });
});