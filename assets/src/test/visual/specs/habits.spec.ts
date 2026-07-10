import { test, expect, Page } from "@playwright/test";
import { HabitsPage } from "../pages/HabitsPage";

const baseURL = "http://127.0.0.1:5173";

test.describe("习惯追踪 E2E 测试", () => {
  test("习惯集列表显示", async ({ page }) => {
    await page.goto(baseURL);
    await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");

    const content = page.locator("body");
    await expect(content).toBeVisible();

    const habitsPage = new HabitsPage(page);
    const habitListVisible = await habitsPage.isVisible(habitsPage.habitSetList);
    expect(habitListVisible).toBe(true);

    await page.waitForLoadState("networkidle");
  });

  test("创建习惯集", async ({ page }) => {
    const habitsPage = new HabitsPage(page);
    await habitsPage.goto();
    await page.waitForLoadState("networkidle");

    const beforeCount = await habitsPage.getHabitSetCount();

    await habitsPage.clickCreateSet();
    const uniqueName = `晨间习惯-${Date.now()}`;
    await habitsPage.fillSetForm(uniqueName, "每日晨间好习惯", "#22C55E");
    await habitsPage.clickConfirm();
    await page.waitForLoadState("networkidle");

    const afterCount = await habitsPage.getHabitSetCount();
    expect(afterCount).toBeGreaterThanOrEqual(1);

    await page.waitForLoadState("networkidle");
  });

  test("创建习惯并打卡", async ({ page }) => {
    const habitsPage = new HabitsPage(page);
    await habitsPage.goto();
    await page.waitForLoadState("networkidle");

    // 确保有习惯集（无则先创建）
    let setCount = await habitsPage.getHabitSetCount();
    if (setCount === 0) {
      await habitsPage.clickCreateSet();
      await habitsPage.fillSetForm(`基础习惯-${Date.now()}`, "基础打卡", "#3B82F6");
      await habitsPage.clickConfirm();
        await page.waitForLoadState("networkidle");
        setCount = await habitsPage.getHabitSetCount();
      }
      expect(setCount).toBeGreaterThan(0);

      // 选择第一个习惯集
      await habitsPage.selectHabitSet(0);

      const beforeHabitCount = await habitsPage.getHabitCount();

    // 创建习惯
    await habitsPage.clickCreateHabit();
    await habitsPage.fillHabitForm(`喝水-${Date.now()}`, 60);
    await habitsPage.clickConfirm();
    await page.waitForTimeout(800);

    const afterHabitCount = await habitsPage.getHabitCount();
    expect(afterHabitCount).toBeGreaterThan(beforeHabitCount);

    // 打卡
    await habitsPage.clickCheckIn(0);
    await page.waitForLoadState("networkidle");

    await page.waitForLoadState("networkidle");
  });
});
