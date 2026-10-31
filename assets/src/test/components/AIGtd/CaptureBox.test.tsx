import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { CaptureBox } from "../../../components/AIGtd/CaptureBox";

vi.mock("../../../utils/i18n", () => ({ t: (key: string) => key }));

describe("CaptureBox", () => {
  it("输入区是多行且可增长，不是单行细条", () => {
    render(<CaptureBox onCapture={async () => {}} />);
    const input = screen.getByTestId("capture-input") as HTMLTextAreaElement;
    expect(input.getAttribute("rows")).toBe("4");
  });

  it("提交按钮用 solid primary（对比度达 AA）", () => {
    const { container } = render(<CaptureBox onCapture={async () => {}} />);
    const btn = screen.getByTestId("aigtd-submit");
    expect(btn.className).toContain("btn-primary");
    expect(container.querySelector(".my-surface-card")).toBeTruthy();
  });
});
