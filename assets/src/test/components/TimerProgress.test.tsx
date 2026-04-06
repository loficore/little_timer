import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { TimerProgress } from "../../components/TimerProgress";

describe("TimerProgress", () => {
  const habitDetail = {
    today_seconds: 900,
    goal_seconds: 1500,
    streak: 5,
  };

  it("正计时模式且有习惯详情时应该渲染进度条", () => {
    render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(screen.getByText((content) => content.includes("今日"))).toBeDefined();
    expect(screen.getByText((content) => content.includes("目标"))).toBeDefined();
  });

  it("倒计时模式应该隐藏进度条", () => {
    const { container } = render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={false}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it("无习惯详情时应该隐藏进度条", () => {
    const { container } = render(
      <TimerProgress
        habitDetail={null}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it("应该显示正确的进度百分比", () => {
    render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(screen.getByText((content) => content.includes("进度"))).toBeDefined();
  });

  it("已结束时应该显示成功进度条样式", () => {
    const { container } = render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={600}
        isFinished={true}
        isStopwatchMode={true}
      />
    );

    const progress = container.querySelector("progress");
    expect(progress?.className).toContain("progress-success");
  });

  it("未结束时应该显示主题进度条样式", () => {
    const { container } = render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    const progress = container.querySelector("progress");
    expect(progress?.className).toContain("progress-primary");
  });

  it("有连胜时应该显示连胜信息", () => {
    render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(screen.getByText((content) => content.includes("天"))).toBeDefined();
  });

  it("无连胜时不应该显示连胜信息", () => {
    const { container } = render(
      <TimerProgress
        habitDetail={{ ...habitDetail, streak: 0 }}
        elapsedSeconds={0}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(container.textContent).not.toContain("🔥");
  });

  it("应该计算累计时间包含已用时间", () => {
    render(
      <TimerProgress
        habitDetail={habitDetail}
        elapsedSeconds={300}
        isFinished={false}
        isStopwatchMode={true}
      />
    );

    expect(screen.getByText((content) => content.includes("进度"))).toBeDefined();
  });
});
