import { Page } from "@playwright/test";

export async function testBackendAPI(page: Page) {
  // Set 1 minute duration
  await page.evaluate(async () => {
    await fetch("http://127.0.0.1:8080/api/timer/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        countdown: { duration_seconds: 60, loop: false, loop_count: 0, loop_interval_seconds: 0 },
      }),
    });
  });

  // Check state with polling
  await page.waitForTimeout(2000);

  const state = await page.evaluate(async () => {
    const r = await fetch("http://127.0.0.1:8080/api/state");
    return await r.json();
  });

  console.log("Current timer state:", state);
  console.log("Expected: time ~60, Expected: mode=countdown");
  console.log("Actual: time=" + state.time + ", mode=" + state.mode);
}
