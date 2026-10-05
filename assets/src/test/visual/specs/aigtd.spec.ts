import { test, expect, request } from "@playwright/test";

const baseURL = "http://127.0.0.1:5173";
const apiURL = "http://127.0.0.1:8080";

const fixedLLMResponse = JSON.stringify({
  title: "准备下周汇报 PPT",
  is_complex: true,
  estimated_minutes: 60,
  subtasks: [
    { title: "列大纲", estimated_minutes: 25 },
    { title: "填数据", estimated_minutes: 30 },
  ],
});

async function navigateToAIGtd(page: import("@playwright/test").Page) {
  // 桌面端:点侧栏入口;移动端(< lg):点底部 nav 入口。两个选择器都存在时
  // 取第一个可见的,避免 strict-mode 报错。
  const sidebarBtn = page.locator('aside [data-testid="nav-aigtd"]');
  const bottomBtn = page.locator('nav[data-testid="bottom-nav"] [data-testid="nav-aigtd"]');
  if (await sidebarBtn.isVisible()) {
    await sidebarBtn.click();
  } else {
    await bottomBtn.click();
  }
  await expect(page.getByPlaceholder(/随手记一件事/)).toBeVisible();
}

test.describe("AI GTD E2E", () => {
  test.afterAll(async () => {
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", { data: { text: "" } });
    await ctx.dispose();
  });

  test("随手记 → AI 拆解 → 子任务出现在卡片里", async ({ page }) => {
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", { data: { text: fixedLLMResponse } });
    await ctx.dispose();

    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");

    await navigateToAIGtd(page);
    await page.getByPlaceholder(/随手记一件事/).fill("准备下周汇报 PPT");
    await page.getByPlaceholder(/随手记一件事/).press("Enter");

    await expect(page.getByText("列大纲")).toBeVisible({ timeout: 10000 });
    await expect(page.getByText("填数据")).toBeVisible({ timeout: 10000 });
  });

  test("LLM 5xx → 卡片显示解析失败 + 重新解析按钮", async ({ page }) => {
    const ctx = await request.newContext({ baseURL: apiURL });
    await ctx.post("/api/aigtd/_test/prompt", { data: { fail_with: 500 } });
    await ctx.dispose();

    await page.goto(baseURL);
    await page.waitForLoadState("networkidle");
    await navigateToAIGtd(page);

    await page.getByPlaceholder(/随手记一件事/).fill("test error path");
    await page.getByPlaceholder(/随手记一件事/).press("Enter");

    await expect(page.getByText(/解析失败/).first()).toBeVisible({ timeout: 10000 });
    await expect(page.getByRole("button", { name: /重新解析/ }).first()).toBeVisible();
  });
});