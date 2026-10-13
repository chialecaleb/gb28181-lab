import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:18181",
    viewport: { width: 1280, height: 800 },
    trace: "off",
    screenshot: "off",
  },
  webServer: {
    command:
      process.platform === "win32"
        ? "bin\\gb28181-lab-server.exe"
        : "bin/gb28181-lab-server",
    cwd: "..",
    url: "http://127.0.0.1:18181",
    reuseExistingServer: !process.env.CI,
    env: {
      ...process.env,
      LAB_DATA_DIR: ".local/ui-test",
      LAB_DEVICE_SIP_BIND: "127.0.0.1:15161",
      LAB_PLATFORM_SIP_BIND: "127.0.0.1:15162",
      LAB_HTTP_BIND: "127.0.0.1:18181",
    },
  },
});
