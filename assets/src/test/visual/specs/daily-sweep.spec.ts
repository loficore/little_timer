/**
 * daily-sweep.spec.ts — GTD Daily Sweep / Review 端到端 E2E。
 *
 * 覆盖 user journey:
 *   1. API 直接建一条顶层 active 任务(绕 AI,确定性,无 reparse 抢 LLM fixture);
 *   2. 注入固定 review LLM 响应(`/api/aigtd/_test/prompt`),proposal 指向真实 task_id;
 *   3. 浏览器 clickNav 到 dailySweep → 点"立即扫一遍" → worker 跑完 today 拉出 proposal-card;
 *   4. UI 点 Adopt → card 消失,后端 version→2 + scheduled_start 落地;
 *   5. 二次 adopt 用**当前** version 作 If-Match → FindAdoption 命中短路返回 idempotent;
 *      最终 version 仍 = 2(证明幂等防住双 bump)。
 *
 * 兼容桌面 (≥lg) 和移动 (<lg) 两种布局:导航按钮在 `<aside>` 与
 * `<nav data-testid="bottom-nav">` 各一份,任一可见即可点击(与 scheduler 同款)。
 *
 * 原被回滚 spec (commit 140f7e3) 的两个根因已修:
 *   - `page.goto("/aigtd")` App 的 page 状态初始化为 timer、不走 URL 路由 → 改用 clickNav;
 *   - 移动端底部 nav 无 dailySweep 入口 → 已在 App.tsx 补齐 nav-dailySweep。
 *
 * Project 匹配:测试名带 "E2E" → 走 mobile-390/412/desktop-1280 三个串行项目。
 *
 * 适配 ConflictGuard:`/api` 下所有 mutation 必须带 `If-Match` 或 body `version`。
 */

import { test, expect, request, type Page } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";
const apiURL = "http://127.0.0.1:8080";

/** 桌面与移动的导航按钮分别落在 <aside> 和 <nav data-testid="bottom-nav">。 */
async function clickNav(page: Page, name: string) {
  const sidebarBtn = page.locator(`aside [data-testid="${name}"]`);
  const bottomBtn = page.locator(`nav[data-testid="bottom-nav"] [data-testid="${name}"]`);
  if (await sidebarBtn.first().isVisible().catch(() => false)) {
    await sidebarBtn.first().click();
  } else {
    await bottomBtn.first().click();
  }
}

test.describe("Daily Sweep E2E", () => {
  test.afterAll(async () => {
    // 收尾:清除 LLM override,免污染后续 specs 的 reparse / 评分 worker。
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: "" },
    });
    await ctx.dispose();
  });

  test("手动触发 → review → adopt → 二次 adopt idempotent 端到端 E2E", async ({ page }) => {
    const runId = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
    const taskTitle = `E2E-DS-${runId}`;

    const apiCtx = await request.newContext({ baseURL: apiURL });

    // 0) 清场:archive 所有现有的顶层非 archived 任务。
    //    LLM override 是 global 的,worker 给所有 review job 返回同一份 proposal
    //    (proposal.task_id = 我们的 taskID);但各 ai_job 的真实 task 各异,adopt
    //    校验要求 proposal.task_id == ai_job.task_id(否则 400 I-4)。只有为"我们
    //    task"入队的那个 job 能被采纳 → 让 StartDailySweep 只为我们的 task 入队,
    //    archive 其他顶层任务,ListTopLevel 排除它们(见 ledger Ruling)。
    const listResp = await apiCtx.get("/api/tasks", { headers: { "If-Match": '"0"' } });
    const existing = (await listResp.json()) as Array<{ id: number; version: number }>;
    for (const t of existing) {
      await apiCtx.patch(`/api/tasks/${t.id}`, {
        headers: { "Content-Type": "application/json", "If-Match": `"${t.version}"` },
        data: { status: "archived" },
      });
    }

    // 1) 走 POST /api/tasks 直接建一条 status=active 顶层任务
    //    (绕 AI,确定性;POST 必带 If-Match '"0"',否则 ConflictGuard 400)
    const create = await apiCtx.post("/api/tasks", {
      headers: { "Content-Type": "application/json", "If-Match": '"0"' },
      data: { title: taskTitle },
    });
    expect(create.status(), "POST /api/tasks").toBe(200);
    const created = await create.json();
    const taskID = created.id as number;

    // 2) GET /api/tasks/:id 反查任务,确认落库 + 拿权威 version(=1)
    //    注意:GET /api/tasks 返回的是**裸数组** [TaskDTO...],不是 { tasks:[...] };
    //    按 id 单取更稳(TaskGet 直接返回 TaskDTO)。
    const getOne = await apiCtx.get(`/api/tasks/${taskID}`, { headers: { "If-Match": '"0"' } });
    const persisted = await getOne.json();
    expect(persisted.title, "任务已落库").toBe(taskTitle);
    expect(persisted.version, "POST /api/tasks 后 version=1").toBe(1);

    // 3) 注入 review LLM fixture:action=reschedule,带真实 task_id + runId 化的 change_id
    const reviewResp = JSON.stringify({
      proposals: [{
        change_id: `e2e-ds-${runId}`,
        task_id: taskID,
        action: "reschedule",
        suggested_scheduled_start: 1735689600,
        suggested_scheduled_end: 1735691400,
        reason: `e2e DS deadline ${runId}`,
      }],
    });
    await apiCtx.post("/api/aigtd/_test/prompt", {
      headers: { "If-Match": '"0"' },
      data: { text: reviewResp },
    });

    // 4) 浏览器:goto 首页 → clickNav 到 dailySweep → 点"立即扫一遍"
    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await clickNav(page, "nav-dailySweep");
    await expect(page.locator('[data-testid="start-btn"]')).toBeVisible({ timeout: 10000 });
    await page.locator('[data-testid="start-btn"]').click();

    // 5) 等 review worker 跑完 + UI 拉到 proposal-card(用 reason 含 runId 过滤,
    //    跨 e2e.db 历史任务也只命中本次)。
    //    useDailySweep.start() 内部只在 worker 完成前 refresh 一次,存在竞速;
    //    test 侧先用 API poll 等 worker 把 proposal 写进 /today,再手动点
    //    refresh-btn 让 UI 拉一次(见 ledger Ruling)。
    const card = page
      .locator('[data-testid="proposal-card"]')
      .filter({ hasText: `e2e DS deadline ${runId}` })
      .first();
    await expect
      .poll(
        async () => {
          const r = await apiCtx.get("/api/daily-sweep/today", { headers: { "If-Match": '"0"' } });
          const b = await r.json();
          return (b.proposals as Array<{ proposal: { change_id: string } }>).some(
            (x) => x.proposal.change_id === `e2e-ds-${runId}`,
          );
        },
        // 60s 容忍链式 e2e 里其他 specs 遗留的 ai_job 让 worker 排队的延迟
        // (standalone 跑通常 < 1s; chain 里可能 20-30s+)。
        { timeout: 60000, intervals: [500] },
      )
      .toBe(true);
    await page.locator('[data-testid="refresh-btn"]').click();
    await expect(card).toBeVisible({ timeout: 10000 });
    await expect(card.locator('[data-testid="action-badge"]')).toHaveText("reschedule");

    // 6) 点 Adopt → card 消失 + 后端落库:version→2,scheduled_start 落地
    await card.locator('[data-testid="adopt-btn"]').click();
    await expect(card).toBeHidden({ timeout: 10000 });

    const after = await apiCtx.get(`/api/tasks/${taskID}`, { headers: { "If-Match": '"0"' } });
    const afterBody = await after.json();
    expect(afterBody.version, "adopt 后 version=2").toBe(2);
    expect(afterBody.scheduled_start, "scheduled_start 落地").toBe(1735689600);

    // 7) 二次 adopt:用**当前** version(2)作 If-Match,FindAdoption 命中应短路;
    //    若 FindAdoption 失效,版本闸门本可通过 → 会再次 bump → version 变 3(失败)。
    const today = await apiCtx.get("/api/daily-sweep/today", { headers: { "If-Match": '"0"' } });
    const todayBody = await today.json();
    const p = (todayBody.proposals as Array<{
      ai_job_id: number; proposal: { change_id: string }; version: number;
    }>).find((x) => x.proposal.change_id === `e2e-ds-${runId}`);
    expect(p, "proposal 仍在 today(is_adopted=true 也会列出来)").toBeTruthy();
    expect(p!.is_adopted, "第一次 adopt 后 server 标 is_adopted=true(today 读 adoptions 表)").toBe(true);

    const adoptAgain = await apiCtx.post("/api/daily-sweep/adopt", {
      headers: { "Content-Type": "application/json", "If-Match": '"2"' },
      data: {
        change_id: p!.proposal.change_id,
        ai_job_id: p!.ai_job_id,
        version: p!.version,
        proposal: p!.proposal,
      },
    });
    expect(adoptAgain.status(), "二次 adopt 仍 200").toBe(200);
    const adoptAgainBody = await adoptAgain.json();
    expect(adoptAgainBody.idempotent, "二次 adopt 走幂等前置").toBe(true);
    expect(adoptAgainBody.version_after, "version_after=2(不再 +1)").toBe(2);

    // 8) 铁证:version 仍 = 2(双 adopt 不会双 bump)
    const final = await apiCtx.get(`/api/tasks/${taskID}`, { headers: { "If-Match": '"0"' } });
    const finalBody = await final.json();
    expect(finalBody.version, "双 adopt 后 version 仍 = 2").toBe(2);

    await apiCtx.dispose();
  });
});
