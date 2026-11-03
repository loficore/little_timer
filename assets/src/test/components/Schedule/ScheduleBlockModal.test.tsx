import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { ScheduleBlockModal } from "../../../components/Schedule/ScheduleBlockModal";
import type { TimelineTask } from "../../../components/Schedule/types";

vi.mock("../../../utils/i18n", () => ({
  t: (key: string) => key,
}));

function at(hour: number, minute: number): number {
  return Math.floor(new Date(2026, 10, 3, hour, minute).getTime() / 1000);
}

const task: TimelineTask = {
  id: 1,
  title: "写周报",
  priority_score: 3,
  scheduled_start: at(9, 0),
  scheduled_end: at(10, 0),
};

describe("ScheduleBlockModal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("用块当前时间初始化，保存时回传新的 unix 秒", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <ScheduleBlockModal
        task={task}
        date="2026-11-03"
        onSave={onSave}
        onRemove={vi.fn()}
        onClose={onClose}
      />,
    );

    const start = screen.getByTestId("block-start") as HTMLInputElement;
    const end = screen.getByTestId("block-end") as HTMLInputElement;
    expect(start.value).toBe("09:00");
    expect(end.value).toBe("10:00");

    fireEvent.input(start, { target: { value: "10:30" } });
    fireEvent.input(end, { target: { value: "11:15" } });
    fireEvent.click(screen.getByTestId("block-save"));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith(at(10, 30), at(11, 15));
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("结束时间不晚于开始时间时拒绝保存", async () => {
    const onSave = vi.fn();
    render(
      <ScheduleBlockModal
        task={task}
        date="2026-11-03"
        onSave={onSave}
        onRemove={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    fireEvent.input(screen.getByTestId("block-end"), { target: { value: "09:00" } });
    expect(screen.getByTestId("block-time-error")).toBeTruthy();
    expect((screen.getByTestId("block-save") as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByTestId("block-save"));
    expect(onSave).not.toHaveBeenCalled();
  });

  it("移除排程走 onRemove", async () => {
    const onRemove = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <ScheduleBlockModal
        task={task}
        date="2026-11-03"
        onSave={vi.fn()}
        onRemove={onRemove}
        onClose={onClose}
      />,
    );

    fireEvent.click(screen.getByTestId("block-remove"));
    await waitFor(() => expect(onRemove).toHaveBeenCalled());
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
