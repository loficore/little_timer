import { test as base, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { TimerPage } from "../pages/TimerPage";
import { HabitsPage } from "../pages/HabitsPage";
import { SettingsPage } from "../pages/SettingsPage";

export type { TestUtils };

export class TestUtils {
  constructor(private page: Page) {}

  async resetTimerState() {
    await this.page.evaluate(async () => {
      const response = await fetch("http://127.0.0.1:8080/api/reset", { method: "POST" });
      return response.ok;
    });
    await this.page.waitForTimeout(300);
  }

  async resetDatabase() {
    await this.page.evaluate(async () => {
      try {
        await fetch("http://127.0.0.1:8080/api/state", { method: "GET" });
      } catch {
        // ignore errors during reset
      }
    });
    await this.page.waitForTimeout(500);
  }

  async getApiState<T = any>(endpoint: string): Promise<T | null> {
    try {
      const response = await this.page.evaluate(async (ep: string) => {
        const res = await fetch(`http://127.0.0.1:8080${ep}`);
        if (!res.ok) return null;
        return res.json();
      }, endpoint);
      return response as T;
    } catch {
      return null;
    }
  }
}

export interface TestContext {
  timerPage: TimerPage;
  habitsPage: HabitsPage;
  settingsPage: SettingsPage;
  utils: TestUtils;
}

const test = base.extend<{
  timerPage: TimerPage;
  habitsPage: HabitsPage;
  settingsPage: SettingsPage;
  utils: TestUtils;
}>({
  timerPage: ({ page }, use) => use(new TimerPage(page)),
  habitsPage: ({ page }, use) => use(new HabitsPage(page)),
  settingsPage: ({ page }, use) => use(new SettingsPage(page)),
  utils: ({ page }, use) => use(new TestUtils(page)),
});

export { test, expect };
export type { Page };