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

  it("renders empty timeline when tasks array is empty", () => {
    const { container } = render(<Timeline tasks={[]} />);
    expect(container.querySelectorAll("[data-task-id]").length).toBe(0);
  });
});