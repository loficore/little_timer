import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { Timeline } from "../../../components/Schedule/Timeline";

vi.mock("../../../utils/i18n", () => ({
  t: (key: string) => key,
}));

interface TestTask {
  id: number;
  title: string;
  priority_score: number;
  scheduled_start: number;
  scheduled_end: number;
  context_tag?: string;
}

describe("Timeline", () => {
  it("renders task titles from props", () => {
    const tasks: TestTask[] = [
      { id: 1, title: "urgent", priority_score: 5, scheduled_start: 0, scheduled_end: 3600 },
      { id: 2, title: "low", priority_score: 1, scheduled_start: 0, scheduled_end: 3600 },
    ];
    render(<Timeline tasks={tasks} />);
    expect(screen.getByText("urgent")).toBeTruthy();
    expect(screen.getByText("low")).toBeTruthy();
  });

  it("maps priority 5 to priority-5 class on the task block", () => {
    const tasks: TestTask[] = [
      { id: 1, title: "urgent", priority_score: 5, scheduled_start: 0, scheduled_end: 3600 },
    ];
    render(<Timeline tasks={tasks} />);
    const block = screen.getByText("urgent").closest("[data-task-id]");
    expect(block).toBeTruthy();
    expect(block?.className).toContain("priority-5");
  });

  it("maps priority 1 to priority-1 class on the task block", () => {
    const tasks: TestTask[] = [
      { id: 2, title: "lo", priority_score: 1, scheduled_start: 0, scheduled_end: 3600 },
    ];
    render(<Timeline tasks={tasks} />);
    const block = screen.getByText("lo").closest("[data-task-id]");
    expect(block?.className).toContain("priority-1");
  });

  it("uses visibly distinct priority classes for all 5 priority levels", () => {
    const tasks: TestTask[] = [1, 2, 3, 4, 5].map((p) => ({
      id: p,
      title: `task-${p}`,
      priority_score: p,
      scheduled_start: 0,
      scheduled_end: 3600,
    }));
    render(<Timeline tasks={tasks} />);
    const classNames = [1, 2, 3, 4, 5].map((p) => {
      const el = screen.getByText(`task-${p}`).closest("[data-task-id]");
      return el?.className ?? "";
    });
    const distinctPriorityClasses = new Set(
      classNames
        .map((c) => c.split(/\s+/).find((tok) => /^priority-[1-5]$/.test(tok)))
        .filter((v): v is string => Boolean(v)),
    );
    expect(distinctPriorityClasses.size).toBe(5);
  });

  it("applies context tint class when context_tag is provided", () => {
    const tasks: TestTask[] = [
      {
        id: 1,
        title: "ctx",
        priority_score: 3,
        scheduled_start: 0,
        scheduled_end: 3600,
        context_tag: "编码",
      },
    ];
    render(<Timeline tasks={tasks} />);
    const block = screen.getByText("ctx").closest("[data-task-id]");
    expect(block?.className).toMatch(/context-/);
  });

  it("shows duration in minutes on the task block", () => {
    const tasks: TestTask[] = [
      { id: 1, title: "d", priority_score: 3, scheduled_start: 0, scheduled_end: 30 * 60 },
    ];
    render(<Timeline tasks={tasks} />);
    expect(screen.getByTestId("task-block-1").textContent).toContain("30m");
  });

  it("不渲染习惯时段占位轨", () => {
    const { container } = render(<Timeline tasks={[]} />);
    expect(container.querySelector("[data-testid='habit-lane']")).toBeNull();
  });

  it("renders empty timeline when tasks array is empty", () => {
    const { container } = render(<Timeline tasks={[]} />);
    expect(container.querySelectorAll("[data-task-id]").length).toBe(0);
  });

  it("不再渲染 habit 占位轨", () => {
    const { container } = render(<Timeline tasks={[]} />);
    expect(container.querySelector('[data-testid="habit-lane"]')).toBeNull();
  });

  describe("时间轴定位(本地分钟换算)", () => {
    /** 2026-11-01 当天 hh:mm 的本地 unix 秒。 */
    const at = (h: number, m = 0) => new Date(2026, 10, 1, h, m, 0).getTime() / 1000;

    it("把 unix 秒换算成当天分钟,再算 left/width", () => {
      // 窗口 06:00–23:00 = 1020 分钟;09:00–10:00 应是 17.647% / 5.882%。
      const tasks: TestTask[] = [
        { id: 1, title: "nine", priority_score: 3, scheduled_start: at(9), scheduled_end: at(10) },
      ];
      render(<Timeline tasks={tasks} />);
      const block = screen.getByTestId("task-block-1");
      expect(block.style.left).toBe("17.647%");
      expect(block.style.width).toBe("5.882%");
    });

    it("早于窗口开始的任务夹到左边界,而不是塌成 0 宽", () => {
      const tasks: TestTask[] = [
        { id: 1, title: "early", priority_score: 3, scheduled_start: at(5), scheduled_end: at(7) },
      ];
      render(<Timeline tasks={tasks} />);
      const block = screen.getByTestId("task-block-1");
      expect(block.style.left).toBe("0.000%");
      expect(block.style.width).toBe("5.882%");
    });

    it("晚于窗口结束的任务贴右边缘", () => {
      const tasks: TestTask[] = [
        { id: 1, title: "late", priority_score: 3, scheduled_start: at(23, 30), scheduled_end: at(23, 45) },
      ];
      render(<Timeline tasks={tasks} />);
      const block = screen.getByTestId("task-block-1");
      expect(block.style.left).toBe("98.5%");
      expect(block.style.width).toBe("1.5%");
    });

    it("贴着窗口结束的短任务保留最小宽度,但不溢出轨道", () => {
      // 22:50–23:00 只有 10 分钟(0.98%),会被抬到 1.5% 最小宽度;
      // 若不回夹 left,99.020% + 1.5% 会越过轨道右边。
      const tasks: TestTask[] = [
        { id: 1, title: "edge", priority_score: 3, scheduled_start: at(22, 50), scheduled_end: at(23) },
      ];
      render(<Timeline tasks={tasks} />);
      const block = screen.getByTestId("task-block-1");
      const left = parseFloat(block.style.left);
      const width = parseFloat(block.style.width);
      expect(width).toBeGreaterThanOrEqual(1.5);
      expect(left + width).toBeLessThanOrEqual(100);
    });
  });
});