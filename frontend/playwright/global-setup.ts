import { chromium, type FullConfig } from '@playwright/test'
import fs from 'node:fs/promises'
import path from 'node:path'

function getRequiredEnv(name: string) {
  const value = process.env[name]
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`Missing required env var: ${name}`)
  }
  return value.trim()
}

export const authStorageStatePath = path.resolve(
  process.cwd(),
  'playwright/.auth/storageState.json',
)

export default async function globalSetup(config: FullConfig) {
  const baseURL =
    process.env.COOKING_APP_BASE_URL ??
    (typeof config.projects[0]?.use?.baseURL === 'string'
      ? config.projects[0].use.baseURL
      : 'http://127.0.0.1:5173')

  const username = getRequiredEnv('COOKING_APP_E2E_USERNAME')
  const password = getRequiredEnv('COOKING_APP_E2E_PASSWORD')

  await fs.mkdir(path.dirname(authStorageStatePath), { recursive: true })

  const browser = await chromium.launch()
  const context = await browser.newContext({
    baseURL,
    viewport: { width: 1280, height: 720 },
    reducedMotion: 'reduce',
    colorScheme: 'dark',
    locale: 'en-US',
  })

  try {
    const page = await context.newPage()
    await page.goto('/login', { waitUntil: 'networkidle' })
    await page.getByLabel('Username').fill(username)
    await page.getByLabel('Password').fill(password)
    await page.getByRole('button', { name: /sign in/i }).click()
    await page.waitForURL('**/recipes', { timeout: 15_000 })
    await context.storageState({ path: authStorageStatePath })
  } finally {
    await context.close()
    await browser.close()
  }
}
