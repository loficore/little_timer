/**
 * daily-sweep.spec.ts — GTD Daily Sweep 端到端 E2E(spec delta Δ2/Δ3 + 基线 §6.3/§6.4)。
 *
 * 覆盖 user journey:
 *   1. 在 /aigtd 随手记任务(走 reparse → 落 active);
 *   2. 通过 API 用真实 task_id 注入 review 响应(/api/aigtd/_test/prompt);
 *   3. 跳到 /daily-sweep,点"立即扫一遍" → 入队 review job → worker 跑完 →
 *      today 拉出 proposals;
 *   4. 在 UI 点 Adopt → card 消失;
 *   5. 通过 API 重复 adopt(列表已空)→ 200 idempotent,task version 仍 +1(只 +1)。
 *
 * 与 scheduler.spec.ts 同套:导航按钮在 <aside>(桌面)与
 * <nav data-testid="bottom-nav">(移动)各一份,任一可见即可点击。
 *
 * Project 匹配:本文件测试名带 "E2E" → 走 mobile-390/412/desktop-1280 三个串行项目。
 */

import { test, expect, request, type Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";
const apiURL = "http://127.0.0.1:8080";

async function clickNav(page: Page, testid: string) {
  const sidebarBtn = page.locator(`aside [data-testid="${testid}"]`);
  const bottomBtn = page.locator(`nav[data-testid="bottom-nav"] [data-testid="${testid}"]`);
  if (await sidebarBtn.first().isVisible().catch(() => false)) {
    await sidebarBtn.first().click();
  } else {
    await bottomBtn.first().click();
  }
}

test.describe("Daily Sweep E2E", () => {
  test.afterAll(async () => {
    // 收尾:清除 LLM override,免污染后续 specs。
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: "" },
    });
    await ctx.dispose();
  });

  test("Daily Sweep 手动触发 → adopt → idempotent 端到端 E2E", async ({ page }) => {
    const runId = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
    const taskTitle = `E2E DS ${runId}`;

    const apiCtx = await request.newContext({ baseURL: apiURL });

    // 1) 在 /aigtd 随手记
    await page.goto("/aigtd");
    const capture = page.getByTestId("capture-input");
    await expect(capture).toBeVisible();
    await capture.fill(taskTitle);
    await capture.press("Enter");
    // 等 worker 把任务落到 active(status=inbox→active 由 reparse 后的 apply 决定)
    await page.waitForTimeout(3000);

    // 2) API 找该 task id(避免 DOM 抓取的脆性)
    const listResp = await apiCtx.get("/api/tasks", { headers: { "If-Match": '"0"' } });
    const listBody = await listResp.json();
    const task = listBody.tasks.find((t: { title: string }) => t.title === taskTitle);
    expect(task, "未找到刚随手记的 task").toBeTruthy();
    const taskID = task.id as number;

    // 3) 注入 review 响应(用真实 taskID)
    const reviewResp = JSON.stringify({
      proposals: [
        {
          change_id: `e2e-ds-${runId}`,
          task_id: taskID,
          action: "reschedule",
          suggested_scheduled_start: 1735689600,
          suggested_scheduled_end: 1735691400,
          reason: `e2e DS deadline ${runId}`,
        },
      ],
    });
    await apiCtx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: reviewResp },
    });

    // 4) 跳到 /daily-sweep,点"立即扫一遍"
    await clickNav(page, "nav-dailySweep");
    await page.waitForSelector('[data-testid="start-btn"]', { timeout: 10000 });
    await page.locator('[data-testid="start-btn"]').click();

    // 等 worker 跑完 + today 拉出 proposal
    const card = page.locator('[data-testid="proposal-card"]').filter({
      hasText: `e2e DS deadline ${runId}`,
    }).first();
    await card.waitFor({ state: "visible", timeout: 20000 });

    // 5) UI 点 Adopt → card 消失
    await card.locator('[data-testid="adopt-btn"]').click();
    await card.waitFor({ state: "detached", timeout: 10000 });

    // 6) 验证落库:task version=1,scheduled_start=1735689600
    const after = await apiCtx.get(`/api/tasks/${taskID}`, { headers: { "If-Match": '"0"' } });
    const afterBody = await after.json();
    expect(afterBody.version, "adopt 后 version=1").toBe(1);
    expect(afterBody.scheduled_start, "adopt 后 scheduled_start 落地").toBe(1735689600);

    // 7) API 重复 adopt 同 change → 200 idempotent,version 仍 =1(只 +1)
    const todayResp = await apiCtx.get("/api/daily-sweep/today", { headers: { "If-Match": '"0"' } });
    const todayBody = await todayResp.json();
    const p = todayBody.proposals?.find(
      (x: { proposal: { change_id: string }; ai_job_id: number }) =>
        x.proposal.change_id === `e2e-ds-${runId}`,
    );
    expect(p, "proposal 仍在 today(列表优化范围外)").toBeTruthy();
    const adoptAgain = await apiCtx.post("/api/daily-sweep/adopt", {
      headers: { "Content-Type": "application/json", "If-Match": '"0"' },
      data: {
        change_id: p.proposal.change_id,
        ai_job_id: p.ai_job_id,
        version: 1, // 当前 server version
        proposal: p.proposal,
      },
    });
    expect(adoptAgain.status()).toBe(200);
    const adoptAgainBody = await adoptAgain.json();
    expect(adoptAgainBody.idempotent).toBe(true);

    // 8) 铁证:version 仍 = 1(双 adopt 不会双 bump)
    const final = await apiCtx.get(`/api/tasks/${taskID}`, { headers: { "If-Match": '"0"' } });
    const finalBody = await final.json();
    expect(finalBody.version, "双 adopt 后 version 仍 = 1(双 bump 防住)").toBe(1);

    await apiCtx.dispose();
  });
});
