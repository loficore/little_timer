import { Page, Locator, expect } from "@playwright/test";

export class BasePage {
  readonly page: Page;
  readonly baseUrl: string;

  constructor(page: Page) {
    this.page = page;
    this.baseUrl = "http://127.0.0.1:5173";
  }

  async goto(path: string = "/") {
    await this.page.goto(`${this.baseUrl}${path}`);
    await this.waitForPageLoad();
  }

  async waitForPageLoad() {
    await this.page.waitForLoadState("networkidle");
  }

  async waitForSelector(selector: string, options?: { timeout?: number; state?: "visible" | "hidden" | "attached" }) {
    await this.page.waitForSelector(selector, {
      timeout: options?.timeout ?? 10000,
      state: options?.state ?? "visible",
    });
  }

  async click(selector: string, options?: { timeout?: number }) {
    await this.waitForSelector(selector);
    await this.page.locator(selector).scrollIntoViewIfNeeded({ block: 'center' });
    // Use JS click to bypass viewport validation
    await this.page.evaluate((sel) => {
      document.querySelector(sel)?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    }, selector);
    await this.page.waitForTimeout(500);
  }

  async fill(selector: string, value: string) {
    await this.waitForSelector(selector);
    await this.page.fill(selector, value);
  }

  async getText(selector: string): Promise<string> {
    await this.waitForSelector(selector);
    return await this.page.textContent(selector) ?? "";
  }

  async getAttribute(selector: string, attribute: string): Promise<string | null> {
    await this.waitForSelector(selector);
    return await this.page.getAttribute(selector, attribute);
  }

  async isVisible(selector: string): Promise<boolean> {
    try {
      await this.waitForSelector(selector, { state: "visible", timeout: 3000 });
      return true;
    } catch {
      return false;
    }
  }

  async isHidden(selector: string): Promise<boolean> {
    try {
      await this.waitForSelector(selector, { state: "hidden", timeout: 3000 });
      return true;
    } catch {
      return false;
    }
  }

  async takeScreenshot(name: string) {
    await this.page.screenshot({ path: `test-results/screenshots/${name}.png` });
  }

  async reload() {
    await this.page.reload();
    await this.waitForPageLoad();
  }

  async pause() {
    await this.page.pause();
  }
}

export { expect };