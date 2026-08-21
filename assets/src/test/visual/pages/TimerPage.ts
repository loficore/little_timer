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

  constructor(page: Page) {
    super(page);
    this.startButton = '[data-testid="timer-start"]';
    this.pauseButton = '[data-testid="timer-pause"]';
    this.resumeButton = '[data-testid="timer-resume"]';
    this.resetButton = '[data-testid="timer-reset"]';
    this.finishButton = '[data-testid="timer-finish"]';
    this.timerDisplay = '[data-testid="timer-display"]';
    this.modeSelector = '[data-testid="mode-selector"]';
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
    // 点击习惯选择按钮打开弹窗
    const habitPickerBtn = this.page.locator('[data-testid="timer-habit-picker"]');
    await habitPickerBtn.click();

    // 等待弹窗完全显示
    await this.page.waitForSelector('.my-surface-modal', { state: 'visible', timeout: 5000 });

    // 点击最后一个习惯按钮（最近创建、位于列表底部）
    // user-journey 测试会新建习惯并需要选中它，因此这一点很重要
    const lastHabit = this.page.locator(`[data-testid^="habit-option-"]`).last();
    await lastHabit.click();

    // 等待弹窗关闭（基于断言，若仍打开会快速失败）
    const modal = this.page.locator('.my-surface-modal');
    try {
      await expect(modal).toBeHidden({ timeout: 5000 });
    } catch {
      // 强制关闭：按 Escape 键
      await this.page.keyboard.press('Escape');
      await this.page.waitForTimeout(300);
      // 回退：遮罩仍可见时点击遮罩
      const backdrop = this.page.locator('.my-overlay-backdrop');
      if (await backdrop.isVisible()) {
        await backdrop.click({ position: { x: 10, y: 10 } });
        await this.page.waitForTimeout(300);
      }
      await expect(modal).toBeHidden({ timeout: 3000 });
    }

    // 稍等片刻让状态稳定
    await this.page.waitForTimeout(300);
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

  /**
   * 通过 API 设置倒计时模式的工作时长（比在 Preact 中走 UI 填写更可靠）。
   * @param minutes 工作时长，单位为分钟（如 5 = 5 分钟 = 300 秒）
   */
  async setWorkDuration(minutes: number) {
    await this.page.evaluate(async (mins) => {
      const seconds = mins * 60;
      await fetch("http://127.0.0.1:8080/api/timer/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          countdown: { duration_seconds: seconds, loop: false, loop_count: 0, loop_interval_seconds: 0 },
        }),
      });
    }, minutes);
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

  /**
   * 轮询后端状态直到计时结束（不依赖 SSE）。
   * 当 SSE 可能不可靠时（如 CI 无头环境）用它替代 waitForTimerFinish。
   */
  async waitForTimerFinishPolling(timeoutMs: number = 30000, intervalMs: number = 500): Promise<boolean> {
    const deadline = Date.now() + timeoutMs;
    let iterations = 0;
    console.log(`[Polling] Waiting up to ${timeoutMs}ms for timer to finish...`);
    while (Date.now() < deadline) {
      iterations++;
      const state = await this.page.evaluate(async () => {
        const r = await fetch("http://127.0.0.1:8080/api/state");
        return r.json();
      });
      const isFinished = state.is_finished || state.time <= 0;
      if (isFinished) {
        console.log(`[Polling] Timer finished in ${iterations * intervalMs}ms (is_finished=${state.is_finished}, time=${state.time})`);
        return true;
      }
      if (iterations % 10 === 0) {
        console.log(`[Polling] Timer not finished yet. Current time: ${state.time}s, is_finished: ${state.is_finished}`);
      }
      await this.page.waitForTimeout(intervalMs);
    }
    console.log(`[Polling] Timeout reached after ${timeoutMs}ms`);
    return false;
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