import { test, expect } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";

test.describe("StatsPage - 图表显示测试", () => {
  test("计时完成后统计页应该正常显示图表", async ({ page }) => {
    // 1. 通过 API 灌入数据 (http://127.0.0.1:8080)
    await page.goto(baseURL);

    const created = await page.evaluate(async () => {
      const today = new Date().toISOString().split("T")[0];

      const setRes = await fetch("http://127.0.0.1:8080/api/habit-sets", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "测试集", description: "test", color: "#6366f1" }),
      });
      const set = await setRes.json();

      const habitRes = await fetch("http://127.0.0.1:8080/api/habits", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ set_id: set.id, name: "测试习惯", goal_seconds: 1500, color: "#6366f1" }),
      });
      const habit = await habitRes.json();

      const sessionRes = await fetch("http://127.0.0.1:8080/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ habit_id: habit.id, duration_seconds: 1500, count: 1, date: today }),
      });
      const session = await sessionRes.json();

      return { set, habit, session };
    });

    // 2. 进入统计页
    await page.locator('[data-testid="nav-stats"]').filter({ visible: true }).first().click();
    await page.waitForLoadState("networkidle");

    // 3. 点击本周范围
    const weekBtn = page.getByRole("button", { name: "本周" });
    if (await weekBtn.isVisible()) {
      await weekBtn.click();
      await page.waitForLoadState("networkidle");
    }

    // 4. 验证总专注时间卡片可见
    const totalCard = page.locator("text=总专注时间");
    await expect(totalCard).toBeVisible();

    // 5. 验证柱状图 canvas 可见（Chart.js 渲染为 canvas）
    const canvas = page.locator("canvas").first();
    await expect(canvas).toBeVisible({ timeout: 5000 });
  });
});
