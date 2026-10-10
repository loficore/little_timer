import { deflateSync } from "zlib";
import {
  contrastRatio,
  decodePng,
  dominantContrast,
  parseHex,
  relativeLuminance,
} from "../../utils/color";

/** 构造无 filter 的最小 8-bit RGBA PNG（测试 decodePng 用） */
const crcTable = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});
const crc32 = (buf: Buffer) => {
  let c = 0xffffffff;
  for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
};
const chunk = (type: string, data: Buffer) => {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([len, body, crc]);
};
const makePng = (pixels: number[][][]) => {
  const h = pixels.length;
  const w = pixels[0].length;
  const raw = Buffer.alloc(h * (1 + w * 4));
  let o = 0;
  for (const row of pixels) {
    raw[o++] = 0; // filter: None
    for (const [r, g, b] of row) {
      raw[o++] = r;
      raw[o++] = g;
      raw[o++] = b;
      raw[o++] = 255;
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 6; // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
};

describe("parseHex / relativeLuminance / contrastRatio", () => {
  test("解析 6 位与 3 位 hex", () => {
    expect(parseHex("#515BD4")).toEqual({ r: 0x51, g: 0x5b, b: 0xd4 });
    expect(parseHex("#fff")).toEqual({ r: 255, g: 255, b: 255 });
  });

  test("白色相对亮度为 1，黑色为 0", () => {
    expect(relativeLuminance(parseHex("#ffffff"))).toBeCloseTo(1);
    expect(relativeLuminance(parseHex("#000000"))).toBeCloseTo(0);
  });

  test("白色对 #515BD4 对比度 5.53:1", () => {
    expect(contrastRatio(parseHex("#ffffff"), parseHex("#515BD4"))).toBeCloseTo(5.53, 1);
  });

  test("对比度对称且自身为 1", () => {
    const a = parseHex("#6d5232");
    const b = parseHex("#1b1f2a");
    expect(contrastRatio(a, b)).toBeCloseTo(contrastRatio(b, a));
    expect(contrastRatio(a, a)).toBeCloseTo(1);
  });
});

describe("dominantContrast", () => {
  const rows = (fg: number[], bg: number[], fgRatio: number) =>
    Array.from({ length: 10 }, () =>
      Array.from({ length: 10 }, (_, i) => (i / 10 < fgRatio ? fg : bg)),
    );

  test("浅字深底：少数堆为 fg", () => {
    const { fg, bg } = dominantContrast(rows([240, 240, 240], [30, 30, 30], 0.2) as never);
    expect(fg.r).toBeGreaterThan(128);
    expect(bg.r).toBeLessThan(128);
  });

  test("深字浅底：少数堆仍为 fg", () => {
    const { fg, bg } = dominantContrast(rows([30, 30, 30], [240, 240, 240], 0.2) as never);
    expect(fg.r).toBeLessThan(128);
    expect(bg.r).toBeGreaterThan(128);
  });

  test("分离出的两堆满足给定对比度", () => {
    const { fg, bg } = dominantContrast(rows([255, 255, 255], [81, 91, 212], 0.25) as never);
    expect(contrastRatio(fg, bg)).toBeGreaterThan(4.5);
  });

  test("稀疏前景（1% 像素）仍可分离", () => {
    const bg = [34, 34, 41];
    const fg = [230, 230, 240];
    const pixels = Array.from({ length: 20 }, (_, y) =>
      Array.from({ length: 20 }, (_, x) => (y === 0 && x < 4 ? fg : bg)),
    );
    const { fg: f, bg: b } = dominantContrast(pixels as never);
    expect(contrastRatio(f, b)).toBeGreaterThan(3);
  });

  test("单色图退化为 fg === bg", () => {
    const { fg, bg } = dominantContrast(
      rows([34, 34, 41], [34, 34, 41], 0.5) as never,
    );
    expect(fg).toEqual(bg);
  });
});

describe("decodePng", () => {
  test("解码 8-bit RGBA 无 filter PNG", () => {
    const png = makePng([
      [
        [255, 0, 0],
        [0, 255, 0],
      ],
      [
        [0, 0, 255],
        [1, 2, 3],
      ],
    ]);
    const { width, height, pixels } = decodePng(png);
    expect([width, height]).toEqual([2, 2]);
    expect(pixels[0][0]).toEqual([255, 0, 0, 255]);
    expect(pixels[1][1]).toEqual([1, 2, 3, 255]);
  });

  test("解码带 Sub/Up filter 的行", () => {
    // 手工构造 filter 字节: 行0=None, 行1=Up(与行0相同像素 → 解码后还原)
    const w = 2;
    const h = 2;
    const raw = Buffer.alloc(h * (1 + w * 4));
    let o = 0;
    raw[o++] = 0;
    for (const p of [[10, 20, 30, 255], [40, 50, 60, 255]]) for (const v of p) raw[o++] = v;
    raw[o++] = 2; // Up: 当前 = 差值(0) + 上一行同列
    for (let i = 0; i < w * 4; i++) raw[o++] = 0;
    const ihdr = Buffer.alloc(13);
    ihdr.writeUInt32BE(w, 0);
    ihdr.writeUInt32BE(h, 4);
    ihdr[8] = 8;
    ihdr[9] = 6;
    const png = Buffer.concat([
      Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
      chunk("IHDR", ihdr),
      chunk("IDAT", deflateSync(raw)),
      chunk("IEND", Buffer.alloc(0)),
    ]);
    const { pixels } = decodePng(png);
    expect(pixels[1][0]).toEqual([10, 20, 30, 255]);
    expect(pixels[1][1]).toEqual([40, 50, 60, 255]);
  });
});
