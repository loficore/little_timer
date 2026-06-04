import { rmSync, existsSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = resolve(dirname(__filename), "..", "..", "..");

const testTmpDir = resolve(__dirname, "../test_tmp");

export default async function globalTeardown() {
  if (existsSync(testTmpDir)) {
    try {
      rmSync(testTmpDir, { recursive: true, force: true });
      console.log("✅ Test temporary directory cleaned up");
    } catch (err) {
      console.warn("⚠️ Failed to clean up test_tmp directory:", err);
    }
  }
}