import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";

interface WallpaperSelectorProps {
  value: string;
  onChange: (wallpaper: string) => void;
}

const GRADIENTS = [
  { id: "", name: "无", value: "" },
  { id: "sunset", name: "日落", value: "linear-gradient(135deg, #f97316 0%, #ec4899 50%, #8b5cf6 100%)" },
  { id: "ocean", name: "海洋", value: "linear-gradient(135deg, #0ea5e9 0%, #14b8a6 50%, #22c55e 100%)" },
  { id: "night", name: "深夜", value: "linear-gradient(135deg, #1e1b4b 0%, #312e81 50%, #4c1d95 100%)" },
  { id: "forest", name: "森林", value: "linear-gradient(135deg, #14532d 0%, #166534 50%, #15803d 100%)" },
  { id: "dawn", name: "黎明", value: "linear-gradient(135deg, #fdf4ff 0%, #f0abfc 50%, #c084fc 100%)" },
  { id: "aurora", name: "极光", value: "linear-gradient(135deg, #134e4a 0%, #0d9488 50%, #2dd4bf 100%)" },
  { id: "coral", name: "珊瑚", value: "linear-gradient(135deg, #7f1d1d 0%, #be123c 50%, #f43f5e 100%)" },
  { id: "mint", name: "薄荷", value: "linear-gradient(135deg, #042f2e 0%, #134e4a 50%, #14b8a6 100%)" },
];

const COLORS = [
  "#121212",
  "#1e1e2e",
  "#2d1b4e",
  "#1b2838",
  "#0f172a",
  "#18181b",
];

export const WallpaperSelector: FunctionalComponent<WallpaperSelectorProps> = ({
  value,
  onChange,
}) => {
  const [wallpaperType, setWallpaperType] = useState<"gradient" | "color" | "image">("gradient");
  const [imageUrl, setImageUrl] = useState("");

  const isGradient = value.startsWith("linear");
  const isColor = !isGradient && value.startsWith("#");
  const isImage = !isGradient && !isColor && value.length > 0;

  useEffect(() => {
    if (isGradient) {
      setWallpaperType("gradient");
    } else if (isColor) {
      setWallpaperType("color");
    } else if (isImage) {
      setWallpaperType("image");
      setImageUrl(value);
    } else {
      setWallpaperType("gradient");
    }
  }, [value, isGradient, isColor, isImage]);

  const handleGradientSelect = (gradientValue: string) => {
    onChange(gradientValue);
  };

  const handleColorSelect = (colorValue: string) => {
    onChange(colorValue);
  };

  const handleImageUrlChange = (url: string) => {
    setImageUrl(url);
    if (url.trim()) {
      onChange(url.trim());
    }
  };

  return (
    <div className="form-control mb-6">
      <label className="label">
        <span className="label-text">壁纸</span>
      </label>

      <div className="tabs tabs-boxed mb-3">
        <button
          type="button"
          className={`tab ${wallpaperType === "gradient" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("gradient")}
        >
          渐变
        </button>
        <button
          type="button"
          className={`tab ${wallpaperType === "color" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("color")}
        >
          纯色
        </button>
        <button
          type="button"
          className={`tab ${wallpaperType === "image" ? "tab-active" : ""}`}
          onClick={() => setWallpaperType("image")}
        >
          图片
        </button>
      </div>

      {wallpaperType === "gradient" && (
        <div className="grid grid-cols-4 gap-2">
          {GRADIENTS.map((g) => (
            <button
              key={g.id}
              type="button"
              className={`h-12 rounded-lg border-2 transition-all ${
                (value === g.value || (!value && !g.value))
                  ? "border-primary ring-2 ring-primary/30"
                  : "border-base-300 hover:border-base-content/30"
              }`}
              style={g.value ? { background: g.value } : { background: "#2a2a2a" }}
              onClick={() => handleGradientSelect(g.value)}
              title={g.name}
            >
              {!g.value && (
                <span className="text-xs text-base-content/50">无</span>
              )}
            </button>
          ))}
        </div>
      )}

      {wallpaperType === "color" && (
        <div className="flex gap-2 flex-wrap">
          {COLORS.map((c) => (
            <button
              key={c}
              type="button"
              className={`w-10 h-10 rounded-full border-2 transition-all ${
                value === c
                  ? "border-primary ring-2 ring-primary/30"
                  : "border-base-300 hover:scale-110"
              }`}
              style={{ backgroundColor: c }}
              onClick={() => handleColorSelect(c)}
            />
          ))}
          <input
            type="color"
            className="w-10 h-10 rounded-full cursor-pointer"
            value={value.startsWith("#") ? value : "#121212"}
            onChange={(e) => handleColorSelect((e.target as HTMLInputElement).value)}
          />
        </div>
      )}

      {wallpaperType === "image" && (
        <div className="space-y-2">
          <input
            type="text"
            className="my-input w-full text-sm"
            placeholder="输入图片 URL..."
            value={imageUrl}
            onInput={(e) => handleImageUrlChange((e.target as HTMLInputElement).value)}
          />
          {imageUrl && (
            <div className="h-20 rounded-lg overflow-hidden bg-base-200">
              <img
                src={imageUrl}
                alt="预览"
                className="w-full h-full object-cover"
                onError={() => {
                  // 忽略图片加载错误
                }}
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
};