#!/usr/bin/env python3
"""从 Android launcher 图标生成 Windows 用的多尺寸 .ico。

源图是 mipmap-xxxhdpi 的 192px PNG（仓库里已有的唯一高清位图）。
ico 由脚本生成而非手工塞二进制，保证可复现：换源图后重跑即可。

用法：python3 scripts/gen-windows-icon.py
"""
import os

from PIL import Image

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "android/app/src/main/res/mipmap-xxxhdpi/ic_launcher.png")
DST = os.path.join(ROOT, "packaging/windows/little-timer.ico")
SIZES = [16, 24, 32, 48, 64, 128, 256]


def main() -> None:
    if not os.path.isfile(SRC):
        raise SystemExit(f"missing source image: {SRC}")
    os.makedirs(os.path.dirname(DST), exist_ok=True)
    src = Image.open(SRC).convert("RGBA")

    # Pillow 的 ICO 保存会跳过大于源图的尺寸（`size[0] > width` 直接 continue），
    # 且 thumbnail 只缩不放。源图是 192px，而 256 是必须的尺寸 —— 先把源图
    # 放大到最大尺寸，再逐个尺寸显式 resize，7 个尺寸才都在。
    frames = [src.resize((s, s), Image.Resampling.LANCZOS) for s in SIZES]
    # base 必须是最大尺寸那张：Pillow 用 base 的宽高过滤尺寸
    # （`size[0] > width` 就跳过该尺寸），base 小了其余尺寸全被丢。
    base = frames[-1]
    base.save(
        DST,
        format="ICO",
        sizes=[(s, s) for s in SIZES],
        append_images=frames[:-1],
        bitmap_format="bmp",
    )
    print(f"wrote {DST} (sizes: {', '.join(str(s) for s in SIZES)})")


if __name__ == "__main__":
    main()
