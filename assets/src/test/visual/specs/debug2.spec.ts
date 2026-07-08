import { test } from '@playwright/test';

test('check bottom-nav exists', async ({ page }) => {
  await page.goto('http://127.0.0.1:5173');
  await page.waitForLoadState('networkidle');
  
  const bottomNav = await page.evaluate(() => {
    const nav = document.querySelector('[data-testid="bottom-nav"]');
    if (!nav) return 'NOT FOUND';
    const rect = nav.getBoundingClientRect();
    return `found: ${rect.width}x${rect.height} at y=${rect.y}`;
  });
  console.log('bottom-nav:', bottomNav);
  
  const navHabits = await page.evaluate(() => {
    const btn = nav?.querySelector('[data-testid="nav-habits"]');
    if (!btn) return 'NOT FOUND';
    const rect = btn.getBoundingClientRect();
    return `found: ${rect.width}x${rect.height}`;
  });
  console.log('nav-habits inside bottom-nav:', navHabits);
  
  // Try direct selector
  const direct = await page.locator('[data-testid="bottom-nav"] [data-testid="nav-habits"]').count();
  console.log('direct selector count:', direct);
});
