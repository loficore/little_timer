import { describe, it, expect } from "vitest";
import { render } from "@testing-library/preact";
import { SettingItem } from "../../components/SettingItem";

describe("SettingItem", () => {
  it("应该渲染 label 和 children", () => {
    const { getByText } = render(
      <SettingItem label="Test Label">
        <input type="text" />
      </SettingItem>
    );

    expect(getByText("Test Label")).toBeTruthy();
    expect(getByText("Test Label").tagName).toBe("LABEL");
  });

  it("应该渲染子元素", () => {
    const { container } = render(
      <SettingItem label="Test Label">
        <input type="text" data-testid="input" />
      </SettingItem>
    );

    expect(container.querySelector('input[data-testid="input"]')).toBeTruthy();
  });

  it("应该正确渲染多个子元素", () => {
    const { container } = render(
      <SettingItem label="Test Label">
        <input type="text" />
        <input type="number" />
        <span>Additional</span>
      </SettingItem>
    );

    expect(container.querySelectorAll("input").length).toBe(2);
    expect(container.querySelector("span")).toBeTruthy();
  });

  it("应该应用正确的类名", () => {
    const { getByText } = render(
      <SettingItem label="Test Label">
        <input type="text" />
      </SettingItem>
    );

    const label = getByText("Test Label");
    expect(label.className).toContain("text-text-primary-dark");
    expect(label.className).toContain("font-medium");
  });

  it("应该正确处理空白子元素", () => {
    const { container } = render(
      <SettingItem label="Test Label">
        {null}
        {undefined}
        {false}
      </SettingItem>
    );

    expect(container.querySelector(".flex-col")).toBeTruthy();
  });
});
