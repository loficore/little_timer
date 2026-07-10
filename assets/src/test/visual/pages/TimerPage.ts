import { Page, expect } from "@playwright/test";
import { BasePage } from "./BasePage";

export class TimerPage extends BasePage {
  readonly startButton: string;
  readonly pauseButton: string;
  readonly resumeButton: string;
  readonly resetButton: string;
  readonly finishButton: string;
  readonly timerDisplay: string;
  readonly modeSelector: string;
  readonly workDurationInput: string;
  readonly restDurationInput: string;
  readonly loopCountInput: string;

  constructor(page: Page) {
    super(page);
    this.startButton = '[data-testid="timer-start"]';
    this.pauseButton = '[data-testid="timer-pause"]';
    this.resumeButton = '[data-testid="timer-resume"]';
    this.resetButton = '[data-testid="timer-reset"]';
    this.finishButton = '[data-testid="timer-finish"]';
    this.timerDisplay = '[data-testid="timer-display"]';
    this.modeSelector = '[data-testid="mode-selector"]';
    this.workDurationInput = '[data-testid="work-duration"]';
    this.restDurationInput = '[data-testid="rest-duration"]';
    this.loopCountInput = '[data-testid="loop-count"]';
  }

  async goto() {
    await super.goto("/");
    await this.page.waitForTimeout(500);
  }

  async clickStart() {
    await this.click(this.startButton);
    await this.page.waitForTimeout(500);
  }

  async clickPause() {
    await this.click(this.pauseButton);
    await this.page.waitForTimeout(500);
  }

  async clickResume() {
    await this.click(this.resumeButton);
    await this.page.waitForTimeout(500);
  }

  async clickReset() {
    await this.click(this.resetButton);
    await this.page.waitForTimeout(500);
  }

  async clickFinish() {
    await this.click(this.finishButton);
    await this.page.waitForTimeout(500);
  }

  async selectHabit() {
    const habitPickerBtn = this.page.locator('[data-testid="timer-habit-picker"]');
    try {
      await habitPickerBtn.click({ timeout: 3000 });
      await this.page.waitForTimeout(500);
      // Target habit buttons in the scrollable content area, not modal header buttons
      const firstHabit = this.page.locator('.my-surface-modal .overflow-y-auto button.my-field-surface').first();
      if (await firstHabit.isVisible({ timeout: 3000 })) {
        await firstHabit.click();
        await this.page.waitForTimeout(300);
      }
    } catch {
      // Habit picker not visible
    }
  }

  async getTimerDisplayText(): Promise<string> {
    return await this.getText(this.timerDisplay);
  }

  async isTimerRunning(): Promise<boolean> {
    return await this.isVisible(this.pauseButton);
  }

  async isTimerStopped(): Promise<boolean> {
    return await this.isVisible(this.startButton);
  }

  async setWorkDuration(seconds: number) {
    if (await this.isVisible(this.workDurationInput)) {
      await this.fill(this.workDurationInput, seconds.toString());
    }
  }

  async setRestDuration(seconds: number) {
    if (await this.isVisible(this.restDurationInput)) {
      await this.fill(this.restDurationInput, seconds.toString());
    }
  }

  async setLoopCount(count: number) {
    if (await this.isVisible(this.loopCountInput)) {
      await this.fill(this.loopCountInput, count.toString());
    }
  }

  async selectMode(mode: "countdown" | "stopwatch") {
    await this.click(this.modeSelector, { timeout: 15000 });
    await this.page.waitForTimeout(300);
    const option = this.page.locator(`[data-testid="mode-option-${mode}"]`);
    if (await option.isVisible()) {
      await option.click();
    }
  }

  async waitForTimerFinish(timeoutMs: number = 30000) {
    const finishButton = this.page.locator(this.finishButton);
    await finishButton.waitFor({ state: "visible", timeout: timeoutMs });
  }

  async getTimerState(): Promise<{
    display: string;
    isRunning: boolean;
    isFinished: boolean;
  }> {
    const display = await this.getTimerDisplayText();
    const isRunning = await this.isTimerRunning();
    const isFinished = await this.isVisible(this.finishButton);
    return { display, isRunning, isFinished };
  }
}