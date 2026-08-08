import { defineConfig, devices } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import os from 'node:os';
import path from 'node:path';

const isCI = Boolean(process.env.CI);
const frontendDir = process.cwd();
const backendDir = path.resolve(frontendDir, '..', 'backend');
const backendPort = '8080';
const frontendPort = '3000';
const baseURL = `http://127.0.0.1:${frontendPort}`;
const databasePath = path.join(
  os.tmpdir(),
  `chatster-playwright-${process.pid}-${randomUUID()}.db`
);

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 2 : 0,
  // Browser projects share one backend process and SQLite fixture; isolate tests while
  // retaining the two-tab concurrency coverage inside the room-isolation test.
  workers: 1,
  timeout: 45_000,
  expect: {
    timeout: 15_000,
  },
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
  ],
  use: {
    baseURL,
    actionTimeout: 10_000,
    navigationTimeout: 30_000,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
  ],
  webServer: [
    {
      command: 'go run .',
      cwd: backendDir,
      url: `http://127.0.0.1:${backendPort}/health`,
      timeout: 120_000,
      reuseExistingServer: !isCI,
      env: {
        CHATSTER_HTTP_ADDR: `127.0.0.1:${backendPort}`,
        CHATSTER_DB_PATH: databasePath,
        CHATSTER_WS_UPGRADE_RPS: '0',
        CHATSTER_MESSAGE_RPS: '0',
      },
    },
    {
      command: `npm start -- --host 127.0.0.1 --port ${frontendPort}`,
      cwd: frontendDir,
      url: baseURL,
      timeout: 120_000,
      reuseExistingServer: !isCI,
      env: {
        VITE_WS_PORT: backendPort,
        VITE_API_PORT: backendPort,
      },
    },
  ],
});
