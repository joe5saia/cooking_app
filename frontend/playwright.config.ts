import { defineConfig } from '@playwright/test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { loadEnvFile } from './src/utils/envFile'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(__dirname, '..')

loadEnvFile({ envFilePath: path.resolve(repoRoot, '.env.dev') })

const baseURL = process.env.COOKING_APP_BASE_URL ?? 'http://127.0.0.1:5173'

export default defineConfig({
  testDir: './playwright',
  outputDir: '../test-results/playwright',
  retries: process.env.CI ? 1 : 0,
  timeout: 60_000,
  expect: {
    timeout: 10_000,
  },
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  globalSetup: './playwright/global-setup',
})
