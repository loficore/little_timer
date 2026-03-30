import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./src/test/visual",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: "http://127.0.0.1:5173",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    colorScheme: "light",
    deviceScaleFactor: 1,
  },
  expect: {
    toHaveScreenshot: {
      maxDiffPixels: 100,
    },
  },
  webServer: {
    command: "bun run dev --host 127.0.0.1 --port 5173",
    url: "http://127.0.0.1:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
  projects: [
    {
      name: "mobile-390",
      use: {
        browserName: "chromium",
        viewport: { width: 390, height: 844 },
      },
    },
    {
      name: "mobile-412",
      use: {
        browserName: "chromium",
        viewport: { width: 412, height: 915 },
      },
    },
  ],
});
