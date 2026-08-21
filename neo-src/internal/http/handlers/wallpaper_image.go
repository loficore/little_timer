// Package handlers —— 壁纸图片处理（UUID 命名 + 压缩）。
//
// 本文件提供 UUID 文件名生成器，以及 WallpaperUpload 与 WallpaperFromURL
// 共用的 image decode/scale/encode 流水线。
package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

var (
	// 任一维度超过 12000px 时返回 ErrWallpaperDimensionsTooLarge。
	ErrWallpaperDimensionsTooLarge = errors.New("wallpaper dimensions exceed 12000px")
	// 图片无法解码时返回 ErrWallpaperDecodeFailed。
	ErrWallpaperDecodeFailed = errors.New("failed to decode wallpaper image")
	// 无法识别的图片格式返回 ErrWallpaperUnsupportedFormat。
	ErrWallpaperUnsupportedFormat = errors.New("unsupported wallpaper format")
)

// newUUIDHex 用 crypto/rand 返回 32 位小写 hex 字符串。不使用第三方 UUID 库。
func newUUIDHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// processWallpaperImage 读取原始图片字节，按需解码并缩放，再重新编码为
// 输出格式。返回编码后的字节、输出文件扩展名以及错误。
//
// 按输入扩展名的行为：
//
//	.jpg / .jpeg / .png / .webp → 解码，长边 >2560 则缩放，再编码
//	.gif / .svg / .bmp            → 直通（原样返回原始字节）
func processWallpaperImage(src io.Reader, ext string) ([]byte, string, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return processDecodeEncode(data, ext)
	case ".gif", ".svg", ".bmp":
		return data, ext, nil
	default:
		return nil, "", ErrWallpaperUnsupportedFormat
	}
}

// processDecodeEncode 负责需要重新压缩的格式的
// decode → scale → encode 流水线。
func processDecodeEncode(data []byte, ext string) ([]byte, string, error) {
	// 1. DecodeConfig —— 先查尺寸，再决定是否解码整图。
	cfg, err := decodeConfig(bytes.NewReader(data), ext)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrWallpaperDecodeFailed, err)
	}
	if cfg.Width > 12000 || cfg.Height > 12000 {
		return nil, "", ErrWallpaperDimensionsTooLarge
	}

	// 2. 解码整图。
	img, err := decodeFull(bytes.NewReader(data), ext)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrWallpaperDecodeFailed, err)
	}

	// 3. 长边超过 2560px 则缩小（保持宽高比）。
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	longEdge := w
	if h > w {
		longEdge = h
	}
	if longEdge > 2560 {
		scale := 2560.0 / float64(longEdge)
		newW := int(float64(w) * scale)
		newH := int(float64(h) * scale)
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}
		scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, bounds, draw.Over, nil)
		img = scaled
	}

	// 4. 编码为对应的输出格式。
	var buf bytes.Buffer
	var outExt string
	switch ext {
	case ".jpg", ".jpeg", ".webp":
		// 三者都输出 JPEG —— webp 源会被转码。
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", fmt.Errorf("encoding jpeg: %w", err)
		}
		outExt = ".jpg"
	case ".png":
		// PNG 输出保留 alpha 通道。
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encoding png: %w", err)
		}
		outExt = ".png"
	default:
		return nil, "", ErrWallpaperUnsupportedFormat
	}

	return buf.Bytes(), outExt, nil
}

// decodeConfig 读取图片尺寸。.webp 直接用 webp 包，因为它没有注册进
// 标准 image 包。
func decodeConfig(r io.Reader, ext string) (image.Config, error) {
	switch ext {
	case ".webp":
		return webp.DecodeConfig(r)
	default:
		cfg, _, err := image.DecodeConfig(r)
		return cfg, err
	}
}

// decodeFull 解码整图。.webp 直接用 webp 包。
func decodeFull(r io.Reader, ext string) (image.Image, error) {
	switch ext {
	case ".webp":
		return webp.Decode(r)
	default:
		img, _, err := image.Decode(r)
		return img, err
	}
}
