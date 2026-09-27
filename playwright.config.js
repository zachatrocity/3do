import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './web/tests', testMatch: '*.spec.js',
  use: { baseURL: 'http://127.0.0.1:18763' },
  webServer: { command: 'python3 -m http.server 18763 --bind 127.0.0.1 --directory web', url: 'http://127.0.0.1:18763' },
});
