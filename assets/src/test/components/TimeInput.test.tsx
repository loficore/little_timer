import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { TimeInput } from "../../components/TimeInput";

describe("TimeInput", () => {
  it("renders three PickerNumberInputs by default", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3661} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox");
    expect(inputs.length).toBe(3);
  });

  it("displays 0:0:0 when value is 0", () => {
    const onChange = vi.fn();
    render(<TimeInput value={0} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    expect(inputs[0].value).toBe("0");
    expect(inputs[1].value).toBe("0");
    expect(inputs[2].value).toBe("0");
  });

  it("displays 1:1:1 when value is 3661 (1h 1m 1s)", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3661} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    expect(inputs[0].value).toBe("1");
    expect(inputs[1].value).toBe("1");
    expect(inputs[2].value).toBe("1");
  });

  it("displays 1:0:0 when value is 3600 (1h 0m 0s)", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3600} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    expect(inputs[0].value).toBe("1");
    expect(inputs[1].value).toBe("0");
    expect(inputs[2].value).toBe("0");
  });

  it("updates internal state when value prop changes", () => {
    const onChange = vi.fn();
    const { rerender } = render(<TimeInput value={60} onChange={onChange} />);

    rerender(<TimeInput value={3661} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    expect(inputs[0].value).toBe("1");
    expect(inputs[1].value).toBe("1");
    expect(inputs[2].value).toBe("1");
  });

  it("calls onChange with total seconds when hours change", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3600} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox");
    const hourInput = inputs[0];

    fireEvent.change(hourInput, { target: { value: "2" } });

    expect(onChange).toHaveBeenCalledWith(7200);
  });

  it("calls onChange with total seconds when minutes change", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3600} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox");
    const minuteInput = inputs[1];

    fireEvent.change(minuteInput, { target: { value: "30" } });

    expect(onChange).toHaveBeenCalledWith(5400);
  });

  it("calls onChange with total seconds when seconds change", () => {
    const onChange = vi.fn();
    render(<TimeInput value={3600} onChange={onChange} />);

    const inputs = screen.getAllByRole("textbox");
    const secondInput = inputs[2];

    fireEvent.change(secondInput, { target: { value: "45" } });

    expect(onChange).toHaveBeenCalledWith(3645);
  });

  it("hides hours picker when showHours is false", () => {
    const onChange = vi.fn();
    render(<TimeInput value={60} onChange={onChange} showHours={false} />);

    const inputs = screen.getAllByRole("textbox");
    expect(inputs.length).toBe(2);
  });

  it("hides minutes picker when showMinutes is false", () => {
    const onChange = vi.fn();
    render(<TimeInput value={60} onChange={onChange} showMinutes={false} />);

    const inputs = screen.getAllByRole("textbox");
    expect(inputs.length).toBe(2);
  });

  it("hides seconds picker when showSeconds is false", () => {
    const onChange = vi.fn();
    render(<TimeInput value={60} onChange={onChange} showSeconds={false} />);

    const inputs = screen.getAllByRole("textbox");
    expect(inputs.length).toBe(2);
  });

  it("clamps hours to maxHours", () => {
    const onChange = vi.fn();
    render(<TimeInput value={0} onChange={onChange} maxHours={0} />);

    const inputs = screen.getAllByRole("textbox");
    const hourInput = inputs[0];

    fireEvent.change(hourInput, { target: { value: "5" } });

    expect(onChange).toHaveBeenCalledWith(0);
  });

  it("displays label when label prop is provided", () => {
    const onChange = vi.fn();
    render(<TimeInput value={0} onChange={onChange} label="Duration" />);

    expect(screen.getByText("Duration")).toBeTruthy();
  });

  it("displays hint when hint prop is provided", () => {
    const onChange = vi.fn();
    render(<TimeInput value={0} onChange={onChange} hint="Enter time" />);

    expect(screen.getByText("Enter time")).toBeTruthy();
  });

  it("renders only wrapper when all show flags are false", () => {
    const onChange = vi.fn();
    const { container } = render(
      <TimeInput value={0} onChange={onChange} showHours={false} showMinutes={false} showSeconds={false} />
    );

    const inputs = container.querySelectorAll("input");
    expect(inputs.length).toBe(0);
  });

  it("handles value larger than 24 hours correctly", () => {
    const onChange = vi.fn();
    render(<TimeInput value={90000} onChange={onChange} maxHours={100} />);

    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    expect(inputs[0].value).toBe("25");
    expect(inputs[1].value).toBe("0");
    expect(inputs[2].value).toBe("0");
  });
});