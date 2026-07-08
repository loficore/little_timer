import { test, expect } from '@playwright/test';

test('no initial goto - just click', async ({ page }) => {
  // No page.goto() - just try to click
  await page.click('[data-testid="nav-habits"]', { timeout: 5000 });
});

test('with initial goto - then click', async ({ page }) => {
  await page.goto('http://127.0.0.1:5173');
  await page.waitForLoadState('networkidle');
  await page.click('[data-testid="nav-habits"]', { timeout: 5000 });
});
