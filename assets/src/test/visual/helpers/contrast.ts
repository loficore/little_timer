import { expect } from "@playwright/test";
import type { Locator } from "@playwright/test";
import { decodePng, dominantContrast, contrastRatio } from "../../../utils/color";

/**
 * 像素级按钮对比度断言：对元素截图，分离前景/背景两堆像素，
 * 校验 WCAG 对比度 ≥ min。
 */
export async function expectButtonContrast(
  locator: Locator,
  min: number,
  label: string,
): Promise<void> {
  const buf = await locator.screenshot();
  const { pixels } = decodePng(buf);
  const { fg, bg } = dominantContrast(pixels);
  const ratio = contrastRatio(fg, bg);
  expect
    .soft(
      ratio,
      `${label} 前景 rgb(${fg.r.toFixed(0)},${fg.g.toFixed(0)},${fg.b.toFixed(0)}) / 背景 rgb(${bg.r.toFixed(0)},${bg.g.toFixed(0)},${bg.b.toFixed(0)})`,
    )
    .toBeGreaterThanOrEqual(min);
}
