import { expect, test } from "@playwright/test";

const navSelector = '[data-testid="bottom-nav"]';

test.describe("移动端底部导航可视回归", () => {
  test.beforeEach(async ({ page }) => {
    // 拦截 API 请求，保证可视测试不依赖本地后端服务状态
    await page.route("**/api/**", async (route) => {
      const url = route.request().url();

      if (url.includes("/api/habit-sets")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (url.includes("/api/habits")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([]),
        });
        return;
      }

      if (url.includes("/api/settings")) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({}),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({}),
      });
    });

    await page.goto("/");

    // 消除动画和过渡带来的截图噪声
    await page.addStyleTag({
      content: `
        *, *::before, *::after {
          animation: none !important;
          transition: none !important;
        }
      `,
    });

    await page.waitForSelector(navSelector);
  });

  test("默认页底部导航样式", async ({ page }) => {
    const nav = page.locator(navSelector);

    await expect(nav).toBeVisible();
    await expect(nav).toHaveScreenshot("bottom-nav-default.png");

    const box = await nav.boundingBox();
    expect(box).not.toBeNull();
    if (!box) {
      return;
    }

    const items = nav.locator("a");
    await expect(items).toHaveCount(3);

    const first = await items.nth(0).boundingBox();
    const second = await items.nth(1).boundingBox();
    const third = await items.nth(2).boundingBox();

    expect(first && second && third).toBeTruthy();
    if (!first || !second || !third) {
      return;
    }

    expect(first.x).toBeLessThan(second.x);
    expect(second.x).toBeLessThan(third.x);
    expect(Math.abs(first.y - second.y)).toBeLessThanOrEqual(2);
    expect(Math.abs(second.y - third.y)).toBeLessThanOrEqual(2);
  });

  test("切换统计页激活态样式", async ({ page }) => {
    await page.getByTestId("nav-stats").click();

    const statsItem = page.getByTestId("nav-stats");
    await expect(statsItem).toHaveClass(/active/);
    await expect(page.locator(navSelector)).toHaveScreenshot("bottom-nav-stats-active.png");
  });

  test("切换设置页激活态样式", async ({ page }) => {
    await page.getByTestId("nav-settings").click();

    const settingsItem = page.getByTestId("nav-settings");
    await expect(settingsItem).toHaveClass(/active/);
    await expect(page.locator(navSelector)).toHaveScreenshot("bottom-nav-settings-active.png");
  });

  test("桌面断点隐藏底部导航", async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 900 });
    await page.reload();

    await expect(page.locator(navSelector)).toBeHidden();
  });
});
