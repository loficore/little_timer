import { describe, it, expect } from "vitest";
import {
  formatDuration,
  formatDurationShort,
  minutesToSeconds,
  secondsToMinutes,
  formatGoalDuration,
  calculateProgress,
  formatDate,
  getToday,
  getDaysAgo,
} from "../../utils/formatters";

describe("formatDuration", () => {
  it("应该格式化 0 秒", () => {
    expect(formatDuration(0)).toBe("00:00");
  });

  it("应该格式化小于 60 秒", () => {
    expect(formatDuration(59)).toBe("00:59");
  });

  it("应该格式化 1 分钟", () => {
    expect(formatDuration(60)).toBe("01:00");
  });

  it("应该格式化小于 1 小时", () => {
    expect(formatDuration(3599)).toBe("59:59");
  });

  it("应该格式化 1 小时", () => {
    expect(formatDuration(3600)).toBe("1:00:00");
  });

  it("应该格式化 1 小时 1 分 1 秒", () => {
    expect(formatDuration(3661)).toBe("1:01:01");
  });

  it("应该格式化 24 小时", () => {
    expect(formatDuration(86400)).toBe("24:00:00");
  });

  it("应该格式化 25 小时", () => {
    expect(formatDuration(90000)).toBe("25:00:00");
  });

  it("负数应该返回负数格式", () => {
    expect(formatDuration(-100)).toBe("-2:-40");
  });

  it("应该转换小数分钟", () => {
    expect(minutesToSeconds(1.5)).toBe(90);
  });
});

describe("formatDurationShort", () => {
  it("应该格式化 0 分钟", () => {
    expect(formatDurationShort(0)).toBe("0m");
  });

  it("应该格式化小于 1 小时", () => {
    expect(formatDurationShort(1800)).toBe("30m");
  });

  it("应该格式化 1 小时", () => {
    expect(formatDurationShort(3600)).toBe("1h 0m");
  });

  it("应该格式化 1 小时 30 分钟", () => {
    expect(formatDurationShort(5400)).toBe("1h 30m");
  });

  it("应该格式化超过 24 小时", () => {
    expect(formatDurationShort(90000)).toBe("25h 0m");
  });
});

describe("minutesToSeconds", () => {
  it("应该转换 0 分钟", () => {
    expect(minutesToSeconds(0)).toBe(0);
  });

  it("应该转换 1 分钟", () => {
    expect(minutesToSeconds(1)).toBe(60);
  });

  it("应该转换 25 分钟", () => {
    expect(minutesToSeconds(25)).toBe(1500);
  });

  it("应该转换小数分钟", () => {
    expect(minutesToSeconds(1.5)).toBe(90);
  });
});

describe("secondsToMinutes", () => {
  it("应该转换 0 秒", () => {
    expect(secondsToMinutes(0)).toBe(0);
  });

  it("应该转换 60 秒", () => {
    expect(secondsToMinutes(60)).toBe(1);
  });

  it("应该转换 90 秒", () => {
    expect(secondsToMinutes(90)).toBe(1);
  });

  it("应该转换 1500 秒", () => {
    expect(secondsToMinutes(1500)).toBe(25);
  });
});

describe("formatGoalDuration", () => {
  it("应该格式化 0 分钟", () => {
    expect(formatGoalDuration(0)).toBe("0 分钟");
  });

  it("应该格式化小于 1 小时", () => {
    expect(formatGoalDuration(1500)).toBe("25 分钟");
  });

  it("应该格式化 1 小时", () => {
    expect(formatGoalDuration(3600)).toBe("1 小时 0 分钟");
  });

  it("应该格式化 1 小时 30 分钟", () => {
    expect(formatGoalDuration(5400)).toBe("1 小时 30 分钟");
  });
});

describe("calculateProgress", () => {
  it("目标为 0 时应返回 0", () => {
    expect(calculateProgress(100, 0)).toBe(0);
  });

  it("负目标应返回 0", () => {
    expect(calculateProgress(100, -100)).toBe(0);
  });

  it("进度为 0 应返回 0", () => {
    expect(calculateProgress(0, 1500)).toBe(0);
  });

  it("50% 进度应返回 50", () => {
    expect(calculateProgress(750, 1500)).toBe(50);
  });

  it("100% 进度应返回 100", () => {
    expect(calculateProgress(1500, 1500)).toBe(100);
  });

  it("超过 100% 应返回 100", () => {
    expect(calculateProgress(2000, 1500)).toBe(100);
  });
});

describe("formatDate", () => {
  it("应该格式化日期为 YYYY-MM-DD", () => {
    const date = new Date("2026-04-06T10:00:00Z");
    expect(formatDate(date)).toBe("2026-04-06");
  });
});

describe("getToday", () => {
  it("应该返回今天的日期字符串", () => {
    const today = getToday();
    const expected = new Date().toISOString().split("T")[0];
    expect(today).toBe(expected);
  });
});

describe("getDaysAgo", () => {
  it("应该返回昨天的日期", () => {
    const yesterday = getDaysAgo(1);
    const date = new Date();
    date.setDate(date.getDate() - 1);
    const expected = date.toISOString().split("T")[0];
    expect(yesterday).toBe(expected);
  });

  it("应该返回 7 天前的日期", () => {
    const daysAgo = getDaysAgo(7);
    const date = new Date();
    date.setDate(date.getDate() - 7);
    const expected = date.toISOString().split("T")[0];
    expect(daysAgo).toBe(expected);
  });

  it("0 天前应返回今天", () => {
    const today = getDaysAgo(0);
    const expected = new Date().toISOString().split("T")[0];
    expect(today).toBe(expected);
  });
});
