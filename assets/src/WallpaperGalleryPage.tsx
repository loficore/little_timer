/**
 * 壁纸图库管理页面
 * 独立的图库管理：列表 / 缩略图 / 上传 / 删除确认（带引用数提示）
 * 图片已由后端压缩（≤2560px），直接使用原图预览，不做缩略图管线
 */

import type { FunctionalComponent } from "preact";
import { Header } from "./components/Header";
import { GalleryContent } from "./components/GalleryContent";
import { t } from "./utils/i18n";

export const WallpaperGalleryPage: FunctionalComponent = () => {
  return (
    <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
      <Header title={t("gallery.title")} showSettings={false} showStats={false} />
      <GalleryContent />
    </div>
  );
};

export default WallpaperGalleryPage;
