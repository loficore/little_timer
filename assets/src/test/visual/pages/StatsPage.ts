import { Page } from "@playwright/test";
import { BasePage } from "./BasePage";

export class StatsPage extends BasePage {
  readonly totalFocusTimeCard: string;
  readonly completionCountCard: string;
  readonly barChartCanvas: string;
  readonly noDataMessage: string;
  readonly todayButton: string;
  readonly weekButton: string;
  readonly monthButton: string;

  constructor(page: Page) {
    super(page);
    this.totalFocusTimeCard = 'text="总专注时间"';
    this.completionCountCard = 'text="完成次数"';
    this.barChartCanvas = "canvas";
    this.noDataMessage = 'text="暂无数据"';
    this.todayButton = 'text="今天"';
    this.weekButton = 'text="本周"';
    this.monthButton = 'text="本月"';
  }

  async goto() {
    await super.goto("/#/stats");
  }

  async clickWeekRange() {
    await this.click(this.weekButton);
    await this.page.waitForTimeout(500);
  }
}
