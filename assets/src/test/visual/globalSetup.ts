import { spawn } from "child_process";
import { existsSync, mkdirSync, rmSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";
import http from "http";

const __filename = fileURLToPath(import.meta.url);
const __dirname = resolve(dirname(__filename), "..", "..", "..");

const testTmpDir = resolve(__dirname, "../test_tmp");
const testDbPath = resolve(testTmpDir, "e2e.db");

export default async function globalSetup() {
  if (!existsSync(testTmpDir)) {
    mkdirSync(testTmpDir, { recursive: true });
  }
  if (existsSync(testDbPath)) {
    rmSync(testDbPath);
  }
  if (existsSync(resolve(testTmpDir, "presets.db"))) {
    rmSync(resolve(testTmpDir, "presets.db"));
  }

  console.log("✅ Test database cleaned up");

  const backendDir = resolve(__dirname, "..", "..", "..");
  spawn("zig", ["build", "-Doptimize=Debug", "run"], {
    cwd: backendDir,
    stdio: ["ignore", "pipe", "pipe"],
    env: { ...process.env, RUST_BACKTRACE: "1" },
  });

  console.log("Backend process started, waiting for port 8080...");

  const checkPort = (retries: number): Promise<boolean> => {
    return new Promise((resolve) => {
      const tryConnect = (attempt: number) => {
        if (attempt >= retries) {
          resolve(false);
          return;
        }
        http.get("http://127.0.0.1:8080/api/state", (res) => {
          resolve(res.statusCode === 200);
        }).on("error", () => {
          setTimeout(() => tryConnect(attempt + 1), 1000);
        });
      };
      tryConnect(0);
    });
  };

  const startupTimeout = setTimeout(() => {
    console.error("❌ Backend startup timeout");
    process.exit(1);
  }, 60000);

  const isReady = await checkPort(30);
  clearTimeout(startupTimeout);

  if (isReady) {
    console.log("✅ Backend is ready on port 8080");
  } else {
    console.error("❌ Backend port 8080 not responding");
    process.exit(1);
  }
}