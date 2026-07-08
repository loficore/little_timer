import { test } from '@playwright/test';

test('verify selectors', async ({ page }) => {
  await page.goto('http://127.0.0.1:5173');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(1000);
  
  const bottomNavCount = await page.locator('[data-testid="bottom-nav"]').count();
  console.log('bottom-nav count:', bottomNavCount);
  
  const navHabitsInBottomNav = await page.locator('[data-testid="bottom-nav"] >> [data-testid="nav-habits"]').count();
  console.log('nav-habits in bottom-nav:', navHabitsInBottomNav);
  
  const allNavHabits = await page.locator('[data-testid="nav-habits"]').count();
  console.log('all nav-habits count:', allNavHabits);
  
  // Check visibility
  const visibleCount = await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).count();
  console.log('visible nav-habits:', visibleCount);
  
  // Try clicking visible one
  if (visibleCount > 0) {
    await page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click({ timeout: 3000 });
    console.log('Click succeeded!');
  }
});
