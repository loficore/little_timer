import { Page } from "@playwright/test";
import { BasePage } from "./BasePage";

export class HabitsPage extends BasePage {
  readonly createSetButton: string;
  readonly createHabitButton: string;
  readonly habitSetList: string;
  readonly habitList: string;
  readonly habitCheckInButton: string;
  readonly setNameInput: string;
  readonly setDescriptionInput: string;
  readonly setColorInput: string;
  readonly habitNameInput: string;
  readonly habitGoalInput: string;
  readonly confirmButton: string;
  readonly cancelButton: string;

  constructor(page: Page) {
    super(page);
    this.createSetButton = '[data-testid="create-habit-set"]';
    this.createHabitButton = '[data-testid="create-habit"]';
    this.habitSetList = '[data-testid="habit-set-list"]';
    this.habitList = '[data-testid="habit-list"]';
    this.habitCheckInButton = '[data-testid="habit-checkin"]';
    this.setNameInput = '[data-testid="set-name-input"]';
    this.setDescriptionInput = '[data-testid="set-description-input"]';
    this.setColorInput = '[data-testid="set-color-input"]';
    this.habitNameInput = '[data-testid="habit-name-input"]';
    this.habitGoalInput = '[data-testid="habit-goal-input"]';
    this.confirmButton = '[data-testid="confirm-button"]';
    this.cancelButton = '[data-testid="cancel-button"]';
  }

  async goto() {
    await this.page.goto(this.baseUrl);
    await this.page.locator('[data-testid="nav-habits"]').filter({ visible: true }).first().click();
    await this.page.waitForLoadState("networkidle");
  }

  async clickCreateSet() {
    await this.click(this.createSetButton);
    await this.page.waitForTimeout(300);
  }

  async clickCreateHabit() {
    await this.click(this.createHabitButton);
    await this.page.waitForTimeout(300);
  }

  async fillSetForm(name: string, description: string, color: string = "#515BD4") {
    if (await this.isVisible(this.setNameInput)) {
      await this.fill(this.setNameInput, name);
    }
    if (await this.isVisible(this.setDescriptionInput)) {
      await this.fill(this.setDescriptionInput, description);
    }
    if (await this.isVisible(this.setColorInput)) {
      await this.fill(this.setColorInput, color);
    }
  }

  async fillHabitForm(name: string, goalSeconds: number, color: string = "#515BD4") {
    if (await this.isVisible(this.habitNameInput)) {
      await this.fill(this.habitNameInput, name);
    }
    if (await this.isVisible(this.habitGoalInput)) {
      await this.fill(this.habitGoalInput, goalSeconds.toString());
    }
  }

  async clickConfirm() {
    await this.click(this.confirmButton);
    await this.page.waitForTimeout(500);
  }

  async clickCancel() {
    await this.click(this.cancelButton);
    await this.page.waitForTimeout(300);
  }

  async clickCheckIn(habitIndex: number = 0) {
    const checkInButtons = this.page.locator(this.habitCheckInButton);
    if (await checkInButtons.count() > habitIndex) {
      await checkInButtons.nth(habitIndex).click();
      await this.page.waitForTimeout(500);
    }
  }

  async getHabitSetCount(): Promise<number> {
    const list = this.page.locator(this.habitSetList);
    if (await list.isVisible()) {
      return await this.page.locator(`${this.habitSetList} > *`).count();
    }
    return 0;
  }

  async getHabitCount(): Promise<number> {
    const list = this.page.locator(this.habitList);
    if (await list.isVisible()) {
      return await this.page.locator(`${this.habitList} > *`).count();
    }
    return 0;
  }

  async selectHabitSet(index: number) {
    const setItems = this.page.locator('[data-testid="habit-set-item"]');
    if (await setItems.count() > index) {
      await setItems.nth(index).click();
      await this.page.waitForTimeout(300);
    }
  }
}