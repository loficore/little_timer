/**
 * scheduler.spec.ts — Scheduler 端到端冒烟(spec §3.1 / §5 / §8)。
 *
 * 覆盖的 user journey:
 *   1. 注入固定 LLM 响应(`/api/aigtd/_test/prompt`);
 *   2. 在 /a 随手记一条带 runId 的任务(避免与共享 e2e.db 中残留任务混淆);
 *   3. 等 worker 处理完,打开编辑面板,验证 5 维评分落到任务行
 *      (priority=4 / urgency=8 / energy=2 / context="编码");
 *   4. 跳到 /s,确认侧栏任务评分面板同步显示 P4 + 编码;
 *   5. 点 "Run Scheduler",确认任务出现在 Timeline 上。
 *
 * 兼容桌面 (≥lg) 和移动 (<lg) 两种布局:导航按钮在 `<aside>` 与
 * `<nav data-testid="bottom-nav">` 各一份,任一可见即可点击。
 *
 * 适配 Task 3 ConflictGuard 的事实:`/api/aigtd/_test/prompt` 也挂在 /api
 * 路由组,mutation 必须带 `If-Match: "0"`;否则中间件 400。
 */

import { test, expect, request, type Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";
const apiURL = "http://127.0.0.1:8080";

/** 桌面与移动的导航按钮分别落在 <aside> 和 <nav data-testid="bottom-nav">。 */
async function clickNav(page: Page, name: "nav-aigtd" | "nav-schedule") {
  const sidebarBtn = page.locator(`aside [data-testid="${name}"]`);
  const bottomBtn = page.locator(`nav[data-testid="bottom-nav"] [data-testid="${name}"]`);
  if (await sidebarBtn.first().isVisible().catch(() => false)) {
    await sidebarBtn.first().click();
  } else {
    await bottomBtn.first().click();
  }
}

test.describe("Scheduler E2E", () => {
  test.afterAll(async () => {
    // 收尾:清除测试覆盖,免污染后续 specs。
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: "" },
    });
    await ctx.dispose();
  });

  test("随手记 → 评分自动填 → SchedulePage 排进 Timeline", async ({ page }) => {
    // 唯一标题:e2e.db 在 project 链中累积任务,固定标题会让过滤器匹配到旧任务;
    // 带上 runId 让所有断言只命中本次捕获的新任务,跨 project 也保持隔离。
    const runId = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
    const taskTitle = `写报告-${runId}`;

    // LLM canned response(spec §5.3 的 4 维评分 + 1 维依赖由 schema_test.go
    // 已锁定):title 复用 taskTitle,_scores 用嵌套形态。
    const fixedLLMResponse = JSON.stringify({
      title: taskTitle,
      is_complex: false,
      estimated_minutes: 30,
      subtasks: [],
      _scores: {
        priority_score: 4,
        urgency_score: 8,
        energy_required: 2,
        context_tag: "编码",
      },
    });

    // 1) 注入固定 LLM 响应。ConflictGuard 要求 mutation 带 If-Match。
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: fixedLLMResponse },
    });
    await ctx.dispose();

    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");

    // 2) 进入 /a 并随手记。
    await clickNav(page, "nav-aigtd");
    const capture = page.getByTestId("capture-input");
    await expect(capture).toBeVisible();
    await capture.fill(taskTitle);
    // CaptureBox 在不按 Shift+Enter 时也会提交;用 Enter 最稳。
    await capture.press("Enter");

    // 3) 等 worker 跑完 + 5 维评分回写到 tasks 行。
    // 用 taskTitle 过滤,只命中本次捕获的那一条,不受 e2e.db 历史任务干扰。
    const card = page
      .locator('[data-testid^="task-card-"]')
      .filter({ hasText: taskTitle })
      .first();
    await expect(card).toBeVisible({ timeout: 10000 });
    // ai_status=pending/processing 时 TaskCard 渲染 ai-spinner;完成后消失。
    await expect(card.getByTestId("ai-spinner")).toHaveCount(0, { timeout: 15000 });

    // 4) 打开编辑面板,用语言无关的方式核对 5 维评分。
    // 语言无关:`aigtd.action_edit` 在 zh="编辑",en="Edit",两边都匹配。
    await card.getByRole("button", { name: /编辑|^Edit$/ }).click();
    const panel = page.locator('[data-testid="score-edit-panel"]');
    await expect(panel).toBeVisible({ timeout: 5000 });
    await expect(panel.getByTestId("priority-value")).toHaveText("4");
    await expect(panel.getByTestId("urgency-value")).toHaveText("8");
    await expect(panel.getByTestId("energy-2")).toBeChecked();
    await expect(panel.getByTestId("context-select")).toHaveValue("编码");
    // 关闭模态(Escape 由 TaskEditModal 监听)。
    await page.keyboard.press("Escape");
    await expect(panel).toBeHidden({ timeout: 3000 });

    // 5) 跳到 /s,确认侧栏「本次任务」评分面板同步。
    await clickNav(page, "nav-schedule");
    const sidebar = page.getByTestId("schedule-tasks");
    await expect(sidebar).toBeVisible({ timeout: 5000 });
    // 定位本次任务的 li(scope 到 taskTitle 行),保证不被历史任务混淆。
    const myTaskRow = sidebar.locator("li").filter({ hasText: taskTitle }).first();
    await expect(myTaskRow).toBeVisible({ timeout: 5000 });
    await expect(myTaskRow.getByText("P4", { exact: true })).toBeVisible();
    // context_tag "编码" 是 div 内联拼接的末段文本,div 整体文本更长,用 substring 匹配。
    await expect(myTaskRow.getByText(/编码/)).toBeVisible();

    // 6) 跑 Scheduler,确认本次任务进入 Timeline。
    await page.getByTestId("run-scheduler").click();
    // Timeline 上每条任务一个 task-block-<id>;按 taskTitle 过滤只取我这次的。
    const block = page
      .locator('[data-testid^="task-block-"]')
      .filter({ hasText: taskTitle })
      .first();
    await expect(block).toBeVisible({ timeout: 10000 });
    // Timeline 块上 P 徽章是 TaskBlock 的稳定结构。
    await expect(block.getByTestId("priority-badge")).toHaveText("P4");
  });
});