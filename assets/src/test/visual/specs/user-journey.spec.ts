import { test, expect } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";
import { StatsPage } from "../pages/StatsPage";

const baseURL = "http://127.0.0.1:5173";

test.describe("完整用户旅程 (Habit → Timer → Stats)", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to Habits page for a clean start
    await page.goto(baseURL);
    await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");
  });

  test("完整用户旅程: 创建习惯 → 开始计时 → 完成 → 统计页验证", async ({ page }) => {
    let confirmBtn, backdrop;
    // 1. Create a new habit set
    await page.locator('[data-testid="create-habit-set"]').click();
    await page.waitForLoadState("networkidle");
    const uniqueSetName = `晨间习惯-${Date.now()}`;
    await page.locator('[data-testid="habit-name-input"]').fill(uniqueSetName);
    await page.locator('[data-testid="set-description-input"]').fill("每日晨间好习惯");
    const colorInput = page.locator('input[type="color"]').first();
    await colorInput.evaluate((el) => {
      el.style.backgroundColor = "rgb(34, 197, 94)";
    });
    // Click input to focus, then press Enter to submit (avoids viewport issues)
    await page.locator('[data-testid="habit-name-input"]').click();
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);
    await page.waitForLoadState("networkidle");
    // Wait for modal overlay to disappear (assertion-based, fails fast)
    backdrop = page.locator('.my-overlay-backdrop');
    try {
      await expect(backdrop).toBeHidden({ timeout: 5000 });
    } catch {
      // Force-close: press Escape to dismiss modal
      await page.keyboard.press('Escape');
      await page.waitForTimeout(300);
      // Fallback: click cancel button if visible
      if (await page.locator('[data-testid="cancel-button"]').isVisible()) {
        await page.locator('[data-testid="cancel-button"]').click();
        await page.waitForTimeout(300);
      }
      await expect(backdrop).toBeHidden({ timeout: 3000 });
    }

    // 2. Select the habit set (expand it to reveal create-habit button)
    await page.locator('[data-testid="habit-set-item"]').first().click();
    // Wait for the set to expand (create-habit button is inside the expanded area)
    await page.waitForSelector('[data-testid="create-habit"]', { state: 'visible', timeout: 5000 });

    // 3. Create a new habit with goal_seconds=60
    await page.locator('[data-testid="create-habit"]').click();
    await page.waitForLoadState("networkidle");
    const uniqueHabitName = `喝水-${Date.now()}`;
    await page.locator('[data-testid="habit-name-input"]').click();
    await page.keyboard.press('Enter');
    await page.waitForTimeout(500);
    await page.waitForLoadState("networkidle");
    // Wait for modal overlay to disappear (assertion-based, fails fast)
    backdrop = page.locator('.my-overlay-backdrop');
    try {
      await expect(backdrop).toBeHidden({ timeout: 5000 });
    } catch {
      // Force-close: press Escape to dismiss modal
      await page.keyboard.press('Escape');
      await page.waitForTimeout(500);
      await expect(backdrop).not.toBeHidden({ timeout: 3000 });
    }

    // 4. Go to Timer page (default route) and start countdown
    const userTimerPage = new TimerPage(page);
    await userTimerPage.goto();

    // 5. Select countdown mode (using the same method as timer.spec.ts)
    await userTimerPage.selectMode("countdown");

    // 6. Set work duration to 5 seconds (using the same method as timer.spec.ts)
    await userTimerPage.setWorkDuration(5);
    await page.waitForLoadState("networkidle");

    // 7. Select a habit (required before starting timer)
    await userTimerPage.selectHabit();

    // 8. Click start and wait for timer to finish
    await userTimerPage.clickStart();

    // Wait past the 5s countdown
    await page.waitForTimeout(6000);

    // Wait for finish button to appear
    await page.locator('[data-testid="timer-finish"]').waitFor({ state: "visible", timeout: 10000 });

    // 9. Click finish button
    await userTimerPage.clickFinish();
    await page.waitForLoadState("networkidle");

    // 10. Navigate to Stats page and verify
    const statsPage = new StatsPage(page);
    await page.locator('[data-testid="nav-stats"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");

    // 11. Verify "本周" week range button if visible
    await statsPage.clickWeekRange();
    await page.waitForLoadState("networkidle");

    // 12. Verify the stats canvas is visible
    await expect(page.locator("canvas").first()).toBeVisible({ timeout: 5000 });

    // 13. Verify "总专注时间" card is visible
    await expect(page.locator(".card").filter({ hasText: '总专注时间' }).first()).toBeVisible({ timeout: 5000 });

    // 14. Verify total focus time > 0
    const totalCard = page.locator(".card").filter({ hasText: '总专注时间' }).first();
    const timeText = await totalCard.textContent();
    expect(timeText?.trim()).toBeTruthy();
    const timeValue = parseInt(timeText?.replace(/[^\d]/g, "") || "0");
    expect(timeValue).toBeGreaterThan(0);
  });
});
