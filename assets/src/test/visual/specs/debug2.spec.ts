import { test } from '@playwright/test';

test('check bottom-nav exists', async ({ page }) => {
  await page.goto('http://127.0.0.1:5173');
  await page.waitForLoadState('networkidle');
  
  const bottomNav = await page.evaluate(() => {
    const nav = document.querySelector('[data-testid="bottom-nav"]');
    if (!nav) return { found: false };
    const rect = nav.getBoundingClientRect();
    const navHabitsBtn = nav?.querySelector('[data-testid="nav-habits"]');
    return {
      found: true,
      width: rect.width,
      height: rect.height,
      y: rect.y,
      navHabits: navHabitsBtn ? {
        found: true,
        width: navHabitsBtn.getBoundingClientRect().width,
        height: navHabitsBtn.getBoundingClientRect().height
      } : { found: false }
    };
  });
  console.log('bottom-nav and nav-habits:', bottomNav);
  

});
