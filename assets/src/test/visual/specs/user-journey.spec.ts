import { test, expect } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";
import { StatsPage } from "../pages/StatsPage";

test.describe("完整用户旅程 (Habit → Timer → Stats)", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to Habits page for a clean start
    await page.goto(baseURL);
    await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");
  });

  test("完整用户旅程: 创建习惯 → 开始计时 → 完成 → 统计页验证", async ({ page }) => {
    // 1. Create a new habit set
    await page.getByRole("button", { name: "+ 新建习惯集" }).click();
    await page.waitForLoadState("networkidle");
    const uniqueSetName = `晨间习惯-${Date.now()}`;
    await page.getByPlaceholder("请输入习惯集名称").fill(uniqueSetName);
    await page.getByPlaceholder("请输入习惯集描述").fill("每日晨间好习惯");
    const colorInput = page.locator('input[type="color"]').first();
    await colorInput.evaluate((el) => {
      el.style.backgroundColor = "rgb(34, 197, 94)";
    });
    await page.getByRole("button", { name: "确认" }).click();
    await page.waitForLoadState("networkidle");

    // 2. Select the habit set
    await page.locator('[class*="card"]').first().click();
    await page.waitForLoadState("networkidle");

    // 3. Create a new habit with goal_seconds=60
    await page.getByRole("button", { name: "+ 新建习惯" }).click();
    await page.waitForLoadState("networkidle");
    const uniqueHabitName = `喝水-${Date.now()}`;
    await page.getByPlaceholder("请输入习惯名称").fill(uniqueHabitName);
    await page.getByPlaceholder("目标时长 (秒)").fill("60");
    await page.getByRole("button", { name: "确认" }).click();
    await page.waitForLoadState("networkidle");

    // 4. Go to Timer page (default route) and start countdown
    const timerPage = new TimerPage(page);
    await timerPage.goto();
    await page.waitForLoadState("networkidle");

    // 5. Select countdown mode
    await page.getByRole("button", { name: /倒计时|Countdown/i }).click();
    await page.waitForLoadState("networkidle");

    // 6. Set work duration to 5 seconds (fast test)
    const workDurationInput = page.getByPlaceholder("工作时长 (秒)");
    await workDurationInput.click();
    await workDurationInput.fill("5");
    await page.waitForLoadState("networkidle");

    // 7. Click start and wait for timer to finish
    await page.getByRole("button", { name: "开始" }).click();

    // Wait past the 5s countdown
    await page.waitForTimeout(6000);

    // Wait for finish button to appear
    await page.getByRole("button", { name: "完成" }).waitFor({ state: "visible", timeout: 10000 });

    // 8. Click finish button
    await page.getByRole("button", { name: "完成" }).click();
    await page.waitForLoadState("networkidle");

    // 9. Navigate to Stats page and verify
    const statsPage = new StatsPage(page);
    await page.locator('[data-testid="nav-stats"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");

    // 12. Verify "本周" week range button if visible
    await page.getByRole("button", { name: "本周" }).click();
    await page.waitForLoadState("networkidle");

    // 13. Verify the stats canvas is visible
    await expect(page.locator("canvas").first()).toBeVisible({ timeout: 5000 });

    // 14. Verify "总专注时间" card is visible
    await expect(page.getByRole("article", { name: "总专注时间" })).toBeVisible();

    // 15. Verify total focus time > 0
    const totalCard = page.getByRole("article", { name: "总专注时间" });
    const timeText = await totalCard.textContent();
    expect(timeText?.trim()).toBeTruthy();
    const timeValue = parseInt(timeText?.replace(/[^\d]/g, "") || "0");
    expect(timeValue).toBeGreaterThan(0);
  });
});
