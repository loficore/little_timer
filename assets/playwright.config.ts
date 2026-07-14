import { defineConfig } from "@playwright/test";
import { dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
  testDir: "./src/test/visual",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
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
      command: "pnpm run dev --host 127.0.0.1 --port 5173",
      url: "http://127.0.0.1:5173",
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
    {
      command: "cd ../neo-src && go run ./cmd/server/ serve --http-only --db-path ../assets/test_tmp/e2e.db",
      url: "http://127.0.0.1:8080/api/state",
      reuseExistingServer: true,
      timeout: 120000,
    },
  ],
  projects: [
    // E2E projects — parallel, filtered to E2E tests only
    {
      name: "mobile-390",
      use: { browserName: "chromium", viewport: { width: 390, height: 844 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      workers: undefined,
    },
    {
      name: "mobile-412",
      use: { browserName: "chromium", viewport: { width: 412, height: 915 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      workers: undefined,
    },
    {
      name: "desktop-1280",
      use: { browserName: "chromium", viewport: { width: 1280, height: 800 } },
      grep: /E2E|完整用户旅程|stopwatch.*journey|Timer 用户旅程/,
      workers: undefined,
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
