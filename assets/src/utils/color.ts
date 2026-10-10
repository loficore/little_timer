import { inflateSync } from "zlib";

export interface Rgb {
  r: number;
  g: number;
  b: number;
}

export const parseHex = (hex: string): Rgb => {
  const h = hex.replace("#", "");
  const full =
    h.length === 3
      ? h
          .split("")
          .map((c) => c + c)
          .join("")
      : h;
  return {
    r: parseInt(full.slice(0, 2), 16),
    g: parseInt(full.slice(2, 4), 16),
    b: parseInt(full.slice(4, 6), 16),
  };
};

const channelLuminance = (v: number) => {
  const s = v / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
};

export const relativeLuminance = (c: Rgb): number =>
  0.2126 * channelLuminance(c.r) +
  0.7152 * channelLuminance(c.g) +
  0.0722 * channelLuminance(c.b);

export const contrastRatio = (a: Rgb, b: Rgb): number => {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
};

/**
 * 从像素堆中分离前景/背景：背景取 WCAG 相对亮度直方图的众数（±1 bin 内
 * 的像素均值）；前景取亮度两端的极值堆（各 0.5% 尾部）中离背景更远一堆
 * 的均值。尾部取 0.5% 分位以命中字形核心像素——更宽的尾部会被边框/
 * 抗锯齿中间调稀释前景色。若极值堆与背景对比 < 1.2 视为单色退化，
 * 返回 fg === bg。pixels 为行优先的 [r,g,b] 或 [r,g,b,a] 数组的数组。
 */
export const dominantContrast = (
  pixels: number[][][],
): { fg: Rgb; bg: Rgb } => {
  const lums: number[] = [];
  const px: Rgb[] = [];
  for (const row of pixels) {
    for (const p of row) {
      if (p.length > 3 && p[3] < 128) continue;
      const c = { r: p[0], g: p[1], b: p[2] };
      lums.push(relativeLuminance(c));
      px.push(c);
    }
  }
  if (px.length === 0) return { fg: { r: 0, g: 0, b: 0 }, bg: { r: 0, g: 0, b: 0 } };

  const avg = (idx: number[]): Rgb => {
    const n = Math.max(idx.length, 1);
    return {
      r: idx.reduce((s, i) => s + px[i].r, 0) / n,
      g: idx.reduce((s, i) => s + px[i].g, 0) / n,
      b: idx.reduce((s, i) => s + px[i].b, 0) / n,
    };
  };

  const bins = new Uint32Array(256);
  const lumBin = lums.map((l) => Math.min(255, Math.round(l * 255)));
  for (const b of lumBin) bins[b]++;
  let bgBin = 0;
  for (let b = 1; b < 256; b++) if (bins[b] > bins[bgBin]) bgBin = b;

  const bgIdx: number[] = [];
  for (let i = 0; i < px.length; i++) if (Math.abs(lumBin[i] - bgBin) <= 1) bgIdx.push(i);
  const bg = avg(bgIdx);

  const order = lumBin.map((_, i) => i).sort((a, b) => lums[a] - lums[b]);
  const tail = Math.max(2, Math.floor(px.length * 0.005));
  const darkAvg = avg(order.slice(0, tail));
  const lightAvg = avg(order.slice(-tail));
  const bgL = relativeLuminance(bg);
  const fg =
    relativeLuminance(lightAvg) - bgL >= bgL - relativeLuminance(darkAvg)
      ? lightAvg
      : darkAvg;
  if (contrastRatio(fg, bg) < 1.2) return { fg: bg, bg };
  return { fg, bg };
};

export interface DecodedPng {
  width: number;
  height: number;
  pixels: number[][][];
}

/** 解码 8-bit RGB/RGBA、非隔行 PNG（VRT 断言 Node 端使用）。 */
export const decodePng = (buf: Buffer): DecodedPng => {
  let pos = 8;
  let width = 0;
  let height = 0;
  let colorType = 0;
  let bitDepth = 0;
  const idat: Buffer[] = [];
  while (pos < buf.length) {
    const len = buf.readUInt32BE(pos);
    const type = buf.toString("ascii", pos + 4, pos + 8);
    const data = buf.subarray(pos + 8, pos + 8 + len);
    if (type === "IHDR") {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      bitDepth = data[8];
      colorType = data[9];
      if (bitDepth !== 8 || (colorType !== 2 && colorType !== 6)) {
        throw new Error(`decodePng: 仅支持 8-bit RGB/RGBA，得到 depth=${bitDepth} color=${colorType}`);
      }
      if (data[12] !== 0) throw new Error("decodePng: 不支持 interlace");
    } else if (type === "IDAT") {
      idat.push(Buffer.from(data));
    } else if (type === "IEND") {
      break;
    }
    pos += 12 + len;
  }
  const channels = colorType === 6 ? 4 : 3;
  const stride = width * channels;
  const raw = inflateSync(Buffer.concat(idat));
  const pixels: number[][][] = [];
  const prev = new Uint8Array(stride);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1));
    const cur = new Uint8Array(stride);
    for (let x = 0; x < stride; x++) {
      const a = x >= channels ? cur[x - channels] : 0;
      const b = prev[x];
      const c = x >= channels ? prev[x - channels] : 0;
      let v: number;
      switch (filter) {
        case 0: v = line[x]; break;
        case 1: v = line[x] + a; break;
        case 2: v = line[x] + b; break;
        case 3: v = line[x] + ((a + b) >> 1); break;
        case 4: {
          const p = a + b - c;
          const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
          v = line[x] + (pa <= pb && pa <= pc ? a : pb <= pc ? b : c);
          break;
        }
        default: throw new Error(`decodePng: 未知 filter ${filter}`);
      }
      cur[x] = v & 0xff;
    }
    prev.set(cur);
    const row: number[][] = [];
    for (let x = 0; x < width; x++) {
      const o = x * channels;
      row.push(channels === 4 ? [cur[o], cur[o + 1], cur[o + 2], cur[o + 3]] : [cur[o], cur[o + 1], cur[o + 2], 255]);
    }
    pixels.push(row);
  }
  return { width, height, pixels };
};
