import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { Modal } from "../../../components/common/Modal";

vi.mock("../../../utils/i18n", () => ({ t: (key: string) => key }));

describe("Modal", () => {
  it("渲染到 document.body（脱离页面局部层叠上下文）", () => {
    const { container } = render(
      <Modal title="编辑任务" onClose={() => {}} testId="shell">
        <p>内容</p>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog.textContent).toContain("编辑任务");
    // portal:弹窗不在 render 的容器里
    expect(container.contains(dialog)).toBe(false);
    expect(document.body.contains(dialog)).toBe(true);
  });

  it("X 按钮关闭", () => {
    const onClose = vi.fn();
    render(<Modal title="标题" onClose={onClose} />);
    fireEvent.click(screen.getByTestId("modal-close"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Escape 关闭", () => {
    const onClose = vi.fn();
    render(<Modal title="标题" onClose={onClose} />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("点击遮罩关闭", () => {
    const onClose = vi.fn();
    render(<Modal title="标题" onClose={onClose} />);
    fireEvent.click(screen.getByTestId("modal-backdrop"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("内容区可滚动，footer 独立于滚动区", () => {
    render(
      <Modal title="标题" onClose={() => {}} footer={<button>保存</button>}>
        <p>内容</p>
      </Modal>,
    );
    expect(screen.getByTestId("modal-body").className).toContain("overflow-y-auto");
    expect(screen.getByTestId("modal-footer").textContent).toContain("保存");
  });
});
