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

  /**
   * 计时器是后端进程级全局状态，用例之间会互相残留。
   * 清理运行中/暂停中的过期计时器，回到「已停止」。
   * 不重新导航，避免与 beforeEach 的加载竞争。
   */
  async resetStaleTimer() {
    if (await this.isVisible(this.pauseButton)) {
      await this.clickPause();
    } else if (await this.isVisible(this.resumeButton)) {
      // 已处于暂停态，直接 reset
    } else if (await this.isVisible(this.startButton)) {
      return; // 已是停止态
    }
    if (await this.isVisible(this.resetButton)) {
      await this.clickReset();
      await this.page.waitForTimeout(300);
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

  /**
   * 通过 API 设置倒计时模式的工作时长（比在 Preact 中走 UI 填写更可靠）。
   * @param minutes 工作时长，单位为分钟（如 5 = 5 分钟 = 300 秒）
   */
  async setWorkDuration(minutes: number) {
    await this.setCountdownSeconds(minutes * 60);
  }

  /** 以秒为单位设置倒计时时长，便于旅程用例用短时长跑完整流程。 */
  async setCountdownSeconds(seconds: number) {
    const ok = await this.page.evaluate(async (secs) => {
      // ConflictGuard (Task 3): timer/config 无 per-row version 语义,在 body 送 version=0。
      // 不加 If-Match header 是为了避免跨源 fetch 触发 CORS 预检
      // (Access-Control-Allow-Headers 不含 If-Match)。
      const r = await fetch("http://127.0.0.1:8080/api/timer/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          version: 0,
          countdown: { duration_seconds: secs, loop: false, loop_count: 0, loop_interval_seconds: 0 },
        }),
      });
      return r.ok;
    }, seconds);
    if (!ok) throw new Error(`设置倒计时时长失败 (duration=${seconds}s)`);
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
    const option = this.page.locator(`[data-testid="mode-selector-option-${mode}"]`);
    await option.waitFor({ state: "visible", timeout: 5000 });
    await option.click();
    await this.page.waitForTimeout(300);
  }

  /**
   * 等待 rAF 至少推进 seconds 秒。
   * 说明：倒计时 loop=false 时 useTimer 到 0 会回卷而不是设 isFinished，
   * 产品的结束方式只有手动点击「完成」按钮（session.finish）。
   * 因此测试里不再需要「等自然结束」，只需等 UI 已经跑起来、按钮已切换到运行态即可。
   */
  async waitRunning(seconds: number) {
    await this.page.waitForTimeout(seconds * 1000);
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