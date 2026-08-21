// Wails 运行时桩 —— 仅用于 Vite 开发模式（桌面）。
// 在 Android / Wails 构建中，真实的 /wails/runtime.js 由运行时注入。
export const Call = { ByID: () => { throw new Error("Wails runtime not available in Vite dev mode"); } };
export const CancellablePromise = class {};
export const Create = {};
