/**
 * Capture baseline UI screenshots (desktop + mobile) for layout/style regression review.
 *
 * Intended usage:
 * - Start the local stack: `make dev-up`
 * - Ensure a user exists (first-time only): `go run ./backend/cmd/cli bootstrap-user ...`
 * - Install Playwright browser (first-time only): `cd frontend && npx playwright install chromium`
 * - Capture screenshots: `cd frontend && npm run ui:baseline`
 *
 * Environment variables:
 * - `COOKING_APP_BASE_URL` (optional): base URL for the running frontend (default: http://localhost:5173)
 * - `COOKING_APP_E2E_STORAGE_STATE` (optional): path to a Playwright storage state JSON to use for auth
 *   - If unset, uses `history/ui-baseline/storageState.json`.
 *   - If the file does not exist, `COOKING_APP_E2E_USERNAME` + `COOKING_APP_E2E_PASSWORD` are required to log in and create it.
 * - `COOKING_APP_E2E_USERNAME` / `COOKING_APP_E2E_PASSWORD` (optional, required only when creating storage state)
 */

import { chromium } from '@playwright/test'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '../..')
const outputDir = path.resolve(repoRoot, 'history/ui-baseline')

const desktopViewport = { width: 1280, height: 720 }
const mobileViewport = { width: 390, height: 844 }

function getEnv(name) {
  const value = process.env[name]
  return typeof value === 'string' && value.trim() ? value.trim() : null
}

function storageStatePath() {
  const fromEnv = getEnv('COOKING_APP_E2E_STORAGE_STATE')
  if (fromEnv) return path.resolve(repoRoot, fromEnv)
  return path.resolve(outputDir, 'storageState.json')
}

async function fileExists(filePath) {
  try {
    await fs.stat(filePath)
    return true
  } catch {
    return false
  }
}

async function ensureStorageState(browser, baseURL) {
  const storagePath = storageStatePath()
  if (await fileExists(storagePath)) return storagePath

  const username = getEnv('COOKING_APP_E2E_USERNAME')
  const password = getEnv('COOKING_APP_E2E_PASSWORD')
  if (!username || !password) {
    throw new Error(
      [
        'Missing auth context for capturing authenticated pages.',
        `Either provide an existing storage state via COOKING_APP_E2E_STORAGE_STATE (${storagePath})`,
        'or set COOKING_APP_E2E_USERNAME and COOKING_APP_E2E_PASSWORD to create one.',
      ].join(' '),
    )
  }

  const context = await browser.newContext({
    viewport: desktopViewport,
    reducedMotion: 'reduce',
  })
  const page = await context.newPage()
  await page.goto(new URL('/login', baseURL).toString(), {
    waitUntil: 'networkidle',
  })
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL('**/recipes', { timeout: 15_000 })

  await context.storageState({ path: storagePath })
  await context.close()
  return storagePath
}

async function captureUnauthed(browser, baseURL) {
  const targets = [
    {
      name: 'login-desktop.png',
      viewport: desktopViewport,
      path: '/login',
    },
    {
      name: 'login-mobile.png',
      viewport: mobileViewport,
      path: '/login',
    },
  ]

  for (const target of targets) {
    const context = await browser.newContext({
      viewport: target.viewport,
      reducedMotion: 'reduce',
    })
    const page = await context.newPage()
    await page.goto(new URL(target.path, baseURL).toString(), {
      waitUntil: 'networkidle',
    })
    await page.screenshot({
      path: path.resolve(outputDir, target.name),
      fullPage: true,
    })
    await context.close()
  }
}

async function captureAuthed(browser, baseURL, storageState) {
  const targets = [
    {
      name: 'recipes-desktop.png',
      viewport: desktopViewport,
      path: '/recipes',
    },
    {
      name: 'recipes-mobile.png',
      viewport: mobileViewport,
      path: '/recipes',
    },
  ]

  for (const target of targets) {
    const context = await browser.newContext({
      viewport: target.viewport,
      reducedMotion: 'reduce',
      storageState,
    })
    const page = await context.newPage()
    await page.goto(new URL(target.path, baseURL).toString(), {
      waitUntil: 'networkidle',
    })
    await page.getByRole('heading', { name: /recipes/i }).waitFor()
    await page.screenshot({
      path: path.resolve(outputDir, target.name),
      fullPage: true,
    })
    await context.close()
  }
}

async function main() {
  const baseURL = getEnv('COOKING_APP_BASE_URL') ?? 'http://127.0.0.1:5173'

  await fs.mkdir(outputDir, { recursive: true })

  const browser = await chromium.launch()
  try {
    await captureUnauthed(browser, baseURL)
    const storageState = await ensureStorageState(browser, baseURL)
    await captureAuthed(browser, baseURL, storageState)
  } finally {
    await browser.close()
  }
}

await main()
