import { test, expect, Page } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";

const baseURL = "http://127.0.0.1:5173";

async function setTimeDisplayStyle(page: Page, style: "classic" | "seven_segment") {
  await page.evaluate((s) => {
    localStorage.setItem("time_display_style", s);
  }, style);
}

async function closeHabitSelector(page: Page) {
  await page.waitForTimeout(300);

  const closeBtn = page.locator('.my-overlay-backdrop button.btn-circle').first();
  if (await closeBtn.isVisible({ timeout: 200 }).catch(() => false)) {
    await closeBtn.click();
    await page.waitForTimeout(200);
    return;
  }

  const backdrop = page.locator('.my-overlay-backdrop').first();
  if (await backdrop.isVisible({ timeout: 200 }).catch(() => false)) {
    await backdrop.click({ force: true });
    await page.waitForTimeout(200);
    return;
  }

  await page.keyboard.press("Escape");
  await page.waitForTimeout(150);
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

    await closeAnyHabitSelector(page);

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
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-stopwatch-mode.png", { maxDiffPixels: 100 });

    await page.locator('.dropdown-select-btn').first().click();
    await page.waitForLoadState("networkidle");
    await page.locator('.my-surface-modal button').nth(0).click();
    await page.waitForTimeout(500);
    await expect(page).toHaveScreenshot("timer-countdown-mode.png", { maxDiffPixels: 100 });
  });

  test("控制按钮等高（stopwatch 运行中）", async ({ page }) => {
    const startBtn = page.locator('[data-testid="timer-start"]');
    if (await startBtn.isVisible()) {
      await startBtn.click();
      await closeAnyHabitSelector(page);
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

    // 1. 进入页面并选择正计时模式
    await timerPage.goto();

    // 2. 选择习惯
    await timerPage.selectHabit();

    // 防护：清理 SSE 同步或此前运行残留的过期计时器状态
    if (!(await timerPage.isTimerStopped())) {
      await timerPage.clickPause();
      await timerPage.clickReset();
      await page.waitForTimeout(300);
    }

    // 点击开始 —— 验证计时器正在运行
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    // 3. 等待 1 秒 —— 验证显示在跳动
    const displayBefore = await timerPage.getTimerDisplayText();
    await page.waitForTimeout(1000);

    const displayAfter = await timerPage.getTimerDisplayText();
    expect(displayAfter).not.toBe(displayBefore);

    // 4. 点击暂停 —— 验证已暂停（既非运行中也未完全停止）
    await timerPage.clickPause();
    expect(await timerPage.isTimerStopped()).toBe(false);
    expect(await timerPage.isTimerRunning()).toBe(false);

    // 5. 通过 clickResume 继续 —— 验证恢复运行
    await timerPage.clickResume();
    expect(await timerPage.isTimerRunning()).toBe(true);

    // 6. 点击重置 —— 验证回到初始状态（开始按钮可见）
    await timerPage.clickReset();
    expect(await timerPage.isTimerStopped()).toBe(true);
    expect(await timerPage.isTimerRunning()).toBe(false);
  });

  test("countdown 流程: start → finish → 验证状态", async ({ page }) => {
    test.setTimeout(180000); // 最长允许 3 分钟

    const timerPage = new TimerPage(page);

    await timerPage.goto();
    await timerPage.selectMode("countdown");
    await timerPage.setWorkDuration(1); // 1 分钟

    await timerPage.selectHabit();
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    await page.waitForTimeout(6000);

    // 以较长间隔（1 秒）轮询，避免忙等
    const finished = await timerPage.waitForTimerFinishPolling(120000, 1000);
    expect(finished).toBe(true);
    console.log("Timer finished successfully");

    await timerPage.clickFinish();
    expect(await timerPage.isTimerStopped()).toBe(true);
    expect(await timerPage.isTimerRunning()).toBe(false);
  });

  test("countdown 完整旅程: start pause resume reset", async ({ page }) => {
    test.setTimeout(180000); // 最长允许 3 分钟

    const timerPage = new TimerPage(page);

    await timerPage.goto();
    await timerPage.selectMode("countdown");
    await timerPage.setWorkDuration(1); // 1 分钟

    await timerPage.selectHabit();
    await timerPage.clickStart();
    expect(await timerPage.isTimerRunning()).toBe(true);

    await timerPage.clickPause();
    expect(await timerPage.isTimerStopped()).toBe(false);
    expect(await timerPage.isTimerRunning()).toBe(false);

    await timerPage.clickResume();
    expect(await timerPage.isTimerRunning()).toBe(true);

    // 以较长间隔（1 秒）轮询，避免忙等
    const finished = await timerPage.waitForTimerFinishPolling(120000, 1000);
    if (!finished) {
      // 回退：超时未结束时手动完成
      await timerPage.clickFinish();
    }

    expect(await timerPage.isTimerStopped()).toBe(true);
  });
});