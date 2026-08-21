import { test, expect } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";
import { StatsPage } from "../pages/StatsPage";

const baseURL = "http://127.0.0.1:5173";

test.describe("完整用户旅程 (Habit → Timer → Stats)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(baseURL);
    await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");
  });

  test("完整用户旅程: 创建习惯 → 开始计时 → 完成 → 统计页验证", async ({ page }) => {
    let confirmBtn, backdrop;
    await page.locator('[data-testid="create-habit-set"]').click();
    await page.waitForLoadState("networkidle");
    const uniqueSetName = `晨间习惯-${Date.now()}`;
    await page.locator('[data-testid="habit-name-input"]').fill(uniqueSetName);
    await page.locator('[data-testid="set-description-input"]').fill("每日晨间好习惯");
    const colorInput = page.locator('input[type="color"]').first();
    await colorInput.evaluate((el) => {
      el.style.backgroundColor = "rgb(34, 197, 94)";
    });
    await page.locator('[data-testid="habit-name-input"]').click();
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);
    await page.waitForLoadState("networkidle");
    backdrop = page.locator('.my-overlay-backdrop');
    try {
      await expect(backdrop).toBeHidden({ timeout: 5000 });
    } catch {
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);
      const cancelBtn = page.locator('[data-testid="cancel-button"]');
      if (await cancelBtn.isVisible()) {
        await cancelBtn.scrollIntoViewIfNeeded();
        await page.evaluate((sel) => {
          const btn = document.querySelector(sel);
          if (btn) (btn as HTMLElement).click();
        }, '[data-testid="cancel-button"]');
        await page.waitForTimeout(300);
      }
      await expect(backdrop).toBeHidden({ timeout: 3000 });
    }

    await page.locator('[data-testid="habit-set-item"]').first().click();
    await page.waitForSelector('[data-testid="create-habit"]', { state: 'visible', timeout: 5000 });

    await page.locator('[data-testid="create-habit"]').click();
    await page.waitForLoadState("networkidle");
    const uniqueHabitName = `喝水-${Date.now()}`;
    await page.locator('[data-testid="habit-name-input"]').click();
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);
    await page.waitForLoadState("networkidle");
    backdrop = page.locator('.my-overlay-backdrop');
    try {
      await expect(backdrop).toBeHidden({ timeout: 5000 });
    } catch {
      await page.keyboard.press('Escape');
      await page.waitForTimeout(500);
      const cancelBtn = page.locator('[data-testid="cancel-button"]');
      if (await cancelBtn.isVisible()) {
        await cancelBtn.scrollIntoViewIfNeeded();
        await page.evaluate((sel) => {
          const btn = document.querySelector(sel);
          if (btn) (btn as HTMLElement).click();
        }, '[data-testid="cancel-button"]');
        await page.waitForTimeout(300);
      }
      await expect(backdrop).toBeHidden({ timeout: 3000 });
    }

    const userTimerPage = new TimerPage(page);
    await userTimerPage.goto();

    await userTimerPage.selectMode("countdown");

    await userTimerPage.setWorkDuration(5);
    await page.waitForLoadState("networkidle");

    await userTimerPage.selectHabit();

    await userTimerPage.clickStart();

    await page.waitForTimeout(6000);

    await page.locator('[data-testid="timer-finish"]').waitFor({ state: "visible", timeout: 10000 });

    await userTimerPage.clickFinish();
    await page.waitForLoadState("networkidle");

    const statsPage = new StatsPage(page);
    await page.locator('[data-testid="nav-stats"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");

    await statsPage.clickWeekRange();
    await page.waitForLoadState("networkidle");

    await expect(page.locator("canvas").first()).toBeVisible({ timeout: 5000 });

    await expect(page.locator(".card").filter({ hasText: '总专注时间' }).first()).toBeVisible({ timeout: 5000 });

    const totalCard = page.locator(".card").filter({ hasText: '总专注时间' }).first();
    const timeText = await totalCard.textContent();
    expect(timeText?.trim()).toBeTruthy();
    const timeValue = parseInt(timeText?.replace(/[^\d]/g, "") || "0");
    expect(timeValue).toBeGreaterThan(0);
  });
});
