import { defineConfig } from "@playwright/test";
import { dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
  testDir: "./src/test/visual",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: undefined,
  reporter: [
    ["list"],
    ["html", { open: "never", outputDir: "playwright-report" }],
  ],
  use: {
    baseURL: "http://127.0.0.1:5173",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    colorScheme: "light",
    deviceScaleFactor: 1,
    setupFiles: ['./src/test/visual/setup.ts'],
  },
  expect: {
    toHaveScreenshot: {
      maxDiffPixels: 100,
    },
  },
  globalSetup: "./src/test/visual/globalSetup.ts",
  globalTeardown: "./src/test/visual/globalTeardown.ts",
  webServer: [
    {
      command: "pnpm exec vite --host 127.0.0.1 --port 5173",
      url: "http://127.0.0.1:5173",
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
      // vite 的 /api 代理目标由 BACKEND_PORT 决定（默认 8013），
      // 而下面启动的 Go 后端固定监听 8080，不注入则所有 /api 请求返回 500。
      env: { BACKEND_PORT: "8080" },
    },
    {
      // CI 里 neo-src/bin/server 已由前置步骤 go build 生成，直接 exec 单一进程；
      // 本地保留 go run 免手动构建。用 `go run` 时 Playwright 只 kill 到 shell wrapper，
      // 真正的 server 子进程可能残留 stdout 管道，导致 CI 结束后进程挂住十几分钟。
      command: process.env.CI
        ? "../neo-src/bin/server serve --http-only --db-path ./test_tmp/e2e.db"
        : "cd ../neo-src && go run ./cmd/server/ serve --http-only --db-path ../assets/test_tmp/e2e.db",
      cwd: __dirname,
      url: "http://127.0.0.1:8080/api/state",
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
  ],
  projects: [
    // E2E projects — 后端只有一个进程级计时器，多项目并行会互相覆盖时钟状态，
    // 通过 dependencies 链强制跨项目串行（mobile-390 → mobile-412 → desktop-1280）。
    // 注意：Playwright 的 workers 只控制单项目内的并行度，跨项目并行需靠 dependencies 或全局 workers=1。
    {
      name: "mobile-390",
      use: { browserName: "chromium", viewport: { width: 390, height: 844 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      workers: 1,
    },
    {
      name: "mobile-412",
      use: { browserName: "chromium", viewport: { width: 412, height: 915 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      dependencies: ["mobile-390"],
      workers: 1,
    },
    {
      name: "desktop-1280",
      use: { browserName: "chromium", viewport: { width: 1280, height: 800 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      dependencies: ["mobile-412"],
      workers: 1,
    },
    // VRT projects — serial, exclude E2E tests
    {
      name: "vrt-mobile-390",
      use: { browserName: "chromium", viewport: { width: 390, height: 844 } },
      grepInvert: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程|用户旅程 E2E/,
      workers: 1,
    },
    {
      name: "vrt-mobile-412",
      use: { browserName: "chromium", viewport: { width: 412, height: 915 } },
      grepInvert: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程|用户旅程 E2E/,
      workers: 1,
    },
    {
      name: "vrt-desktop-1280",
      use: { browserName: "chromium", viewport: { width: 1280, height: 800 } },
      grepInvert: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程|用户旅程 E2E/,
      workers: 1,
    },
  ],
});
