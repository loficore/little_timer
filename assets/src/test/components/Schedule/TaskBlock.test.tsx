import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { TaskBlock } from "../../../components/Schedule/TaskBlock";
import type { TimelineTask } from "../../../components/Schedule/types";

function at(hour: number, minute: number): number {
  return Math.floor(new Date(2026, 10, 3, hour, minute).getTime() / 1000);
}

const task: TimelineTask = {
  id: 1,
  title: "写周报",
  priority_score: 3,
  scheduled_start: at(9, 0),
  scheduled_end: at(10, 0),
  context_tag: "写作",
};

describe("TaskBlock", () => {
  it("时间块可点击，回调把整块数据交给上层编辑", () => {
    const onSelect = vi.fn();
    render(
      <TaskBlock task={task} windowStartMin={360} windowEndMin={1380} onSelect={onSelect} />,
    );

    fireEvent.click(screen.getByTestId("task-block-1"));
    expect(onSelect).toHaveBeenCalledWith(task);
  });

  it("没有 onSelect 时仍然渲染（只读预览）", () => {
    render(<TaskBlock task={task} windowStartMin={360} windowEndMin={1380} />);
    expect(screen.getByTestId("task-block-1").textContent).toContain("写周报");
  });
});
