import { test } from '@playwright/test';

// Clear localStorage before each test to ensure clean state
test.beforeEach(async ({ page }) => {
  await page.evaluate(() => localStorage.clear());
});
