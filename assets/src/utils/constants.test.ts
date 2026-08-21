import { describe, it, expect } from 'vitest';
import { isAllowedWallpaperUrl } from './constants';

describe('isAllowedWallpaperUrl', () => {
  // 允许：imgur.com 子域及精确域名
  it('allows imgur.com URLs', () => {
    expect(isAllowedWallpaperUrl('https://i.imgur.com/photo.png')).toBe(true);
  });

  it('allows unsplash.com URLs', () => {
    expect(isAllowedWallpaperUrl('https://images.unsplash.com/photo.png')).toBe(true);
  });

  it('allows picsum.photos URLs', () => {
    expect(isAllowedWallpaperUrl('https://picsum.photos/id/237/200/300')).toBe(true);
  });

  // 允许：本地路径
  it('allows local / paths', () => {
    expect(isAllowedWallpaperUrl('/wallpapers/bg.png')).toBe(true);
    expect(isAllowedWallpaperUrl('/images/custom.jpg')).toBe(true);
  });

  // 拒绝：外部恶意域名
  it('rejects evil.com URLs', () => {
    expect(isAllowedWallpaperUrl('http://evil.com/evil.png')).toBe(false);
  });

  it('rejects arbitrary external domains', () => {
    expect(isAllowedWallpaperUrl('https://example.com/image.png')).toBe(false);
    expect(isAllowedWallpaperUrl('http://google.com/image.png')).toBe(false);
  });

  // 拒绝：相对路径（无前导 /）
  it('rejects relative paths', () => {
    expect(isAllowedWallpaperUrl('images/bg.png')).toBe(false);
    expect(isAllowedWallpaperUrl('./wallpaper.jpg')).toBe(false);
  });

  // 拒绝：非法 URL
  it('rejects non-http protocols', () => {
    expect(isAllowedWallpaperUrl('file:///etc/passwd')).toBe(false);
    expect(isAllowedWallpaperUrl('javascript:alert(1)')).toBe(false);
  });

  it('rejects empty string', () => {
    expect(isAllowedWallpaperUrl('')).toBe(false);
  });
});
