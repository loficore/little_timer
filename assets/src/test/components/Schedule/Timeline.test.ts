import { describe, it, expect } from "vitest";
import { pickTickHours } from "../../../components/Schedule/Timeline";

describe("pickTickHours", () => {
  it("390px 只保留 6 小时刻度", () => {
    expect(pickTickHours(390)).toEqual([6, 12, 18]);
  });
  it("768px 保留 3 小时刻度", () => {
    expect(pickTickHours(768)).toEqual([6, 9, 12, 15, 18, 21]);
  });
  it("1280px 保留每小时刻度", () => {
    expect(pickTickHours(1280).length).toBe(18);
  });
});
