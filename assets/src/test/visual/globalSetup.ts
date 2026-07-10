import { existsSync, mkdirSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";
import http from "http";

const __filename = fileURLToPath(import.meta.url);
const __dirname = resolve(dirname(__filename), "..", "..", "..");

const testTmpDir = resolve(__dirname, "../test_tmp");

async function setThemeSettings(): Promise<void> {
  const settingsPayload = JSON.stringify({
    basic: {
      timezone: 8,
      language: "ZH",
      default_mode: "countdown",
      theme_mode: "light",
      wallpaper: "paper-cream",
    },
    clock_defaults: {
      countdown: { duration_seconds: 1500, loop: false },
      stopwatch: { max_seconds: 86400 },
    },
  });

  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        hostname: "127.0.0.1",
        port: 8080,
        path: "/api/settings",
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(settingsPayload),
        },
      },
      (res) => {
        let body = "";
        res.on("data", (chunk) => (body += chunk));
        res.on("end", () => {
          if (res.statusCode === 200) {
            resolve();
          } else {
            reject(new Error(`Settings API returned ${res.statusCode}: ${body}`));
          }
        });
      }
    );
    req.on("error", reject);
    req.write(settingsPayload);
    req.end();
  });
}

async function waitForBackend(): Promise<void> {
  const maxRetries = 30;
  for (let i = 0; i < maxRetries; i++) {
    try {
      await new Promise<void>((resolve, reject) => {
        http.get("http://127.0.0.1:8080/api/state", (res) => {
          if (res.statusCode === 200) resolve();
          else reject(new Error(`Status ${res.statusCode}`));
        }).on("error", reject);
      });
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
  throw new Error("Backend not ready after 30 seconds");
}

async function createTestHabitSet(): Promise<void> {
  const habitSetPayload = JSON.stringify({
    name: "Test Habit Set",
    description: "Default set for E2E tests",
    color: "#FFFFFF",
  });

  await new Promise<void>((resolve, reject) => {
    const req = http.request(
      {
        hostname: "127.0.0.1",
        port: 8080,
        path: "/api/habit-sets",
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(habitSetPayload),
        },
      },
      (res) => {
        let body = "";
        res.on("data", (chunk) => (body += chunk));
        res.on("end", () => {
          if (res.statusCode === 200) {
            resolve();
          } else {
            reject(new Error(`HabitSet API returned ${res.statusCode}: ${body}`));
          }
        });
      }
    );
    req.on("error", reject);
    req.write(habitSetPayload);
    req.end();
  });
}

async function createTestHabit(): Promise<void> {
  const habitPayload = JSON.stringify({
    set_id: 1,
    name: "Test Habit",
    goal_seconds: 3600, // 1 hour default
    color: "#FF5733",
  });

  await new Promise<void>((resolve, reject) => {
    const req = http.request(
      {
        hostname: "127.0.0.1",
        port: 8080,
        path: "/api/habits",
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(habitPayload),
        },
      },
      (res) => {
        let body = "";
        res.on("data", (chunk) => (body += chunk));
        res.on("end", () => {
          if (res.statusCode === 200) {
            resolve();
          } else {
            reject(new Error(`Habit API returned ${res.statusCode}: ${body}`));
          }
        });
      }
    );
    req.on("error", reject);
    req.write(habitPayload);
    req.end();
  });
}

export default async function globalSetup() {
  if (!existsSync(testTmpDir)) {
    mkdirSync(testTmpDir, { recursive: true });
  }

  console.log("✅ Test directory ready");

  await waitForBackend();
  console.log("✅ Backend ready");

  await setThemeSettings();
  console.log("✅ Test theme settings applied (light + paper-cream)");

  await createTestHabitSet();
  console.log("✅ Test Habit Set created");

  await createTestHabit();
  console.log("✅ Test Habit created");
}