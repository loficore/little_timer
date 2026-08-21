import { test } from '@playwright/test';

// 每个测试前清空 localStorage，保证干净的初始状态
test.beforeEach(async ({ page }) => {
  await page.evaluate(() => localStorage.clear());
});
