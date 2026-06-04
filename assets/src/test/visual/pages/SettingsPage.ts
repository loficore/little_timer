import { Page } from "@playwright/test";
import { BasePage } from "./BasePage";

export class SettingsPage extends BasePage {
  readonly basicTab: string;
  readonly countdownTab: string;
  readonly stopwatchTab: string;
  readonly themeToggle: string;
  readonly languageSelect: string;
  readonly saveButton: string;
  readonly resetButton: string;
  readonly successMessage: string;

  constructor(page: Page) {
    super(page);
    this.basicTab = '[data-testid="settings-tab-basic"]';
    this.countdownTab = '[data-testid="settings-tab-countdown"]';
    this.stopwatchTab = '[data-testid="settings-tab-stopwatch"]';
    this.themeToggle = '[data-testid="theme-toggle"]';
    this.languageSelect = '[data-testid="language-select"]';
    this.saveButton = '[data-testid="settings-save"]';
    this.resetButton = '[data-testid="settings-reset"]';
    this.successMessage = '[data-testid="save-success"]';
  }

  async goto() {
    await super.goto("/#/settings");
  }

  async clickBasicTab() {
    await this.click(this.basicTab);
    await this.page.waitForTimeout(300);
  }

  async clickCountdownTab() {
    await this.click(this.countdownTab);
    await this.page.waitForTimeout(300);
  }

  async clickStopwatchTab() {
    await this.click(this.stopwatchTab);
    await this.page.waitForTimeout(300);
  }

  async toggleTheme() {
    await this.click(this.themeToggle);
    await this.page.waitForTimeout(500);
  }

  async selectLanguage(lang: string) {
    if (await this.isVisible(this.languageSelect)) {
      await this.page.selectOption(this.languageSelect, lang);
      await this.page.waitForTimeout(300);
    }
  }

  async clickSave() {
    await this.click(this.saveButton);
    await this.page.waitForTimeout(500);
  }

  async clickReset() {
    await this.click(this.resetButton);
    await this.page.waitForTimeout(500);
  }

  async isSaveSuccessVisible(): Promise<boolean> {
    return await this.isVisible(this.successMessage);
  }

  async getCurrentTheme(): Promise<"dark" | "light"> {
    const html = await this.page.locator("html");
    const classes = await html.getAttribute("class");
    return classes?.includes("light-mode") ? "light" : "dark";
  }
}