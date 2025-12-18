import { expect, test } from '@playwright/test'

import { authStorageStatePath } from './global-setup'

const viewports = [
  { name: 'desktop', viewport: { width: 1280, height: 720 } },
  { name: 'mobile', viewport: { width: 390, height: 844 } },
]

function getBaseURL() {
  const baseURL = test.info().project.use.baseURL
  if (typeof baseURL !== 'string' || baseURL.trim() === '') {
    throw new Error('playwright config must set use.baseURL')
  }
  return baseURL
}

test('visual snapshots: /login (unauthenticated)', async ({ browser }) => {
  const baseURL = getBaseURL()

  for (const v of viewports) {
    const context = await browser.newContext({
      baseURL,
      viewport: v.viewport,
      reducedMotion: 'reduce',
      colorScheme: 'dark',
      locale: 'en-US',
      timezoneId: 'UTC',
    })

    try {
      const page = await context.newPage()
      await page.goto('/login', { waitUntil: 'networkidle' })
      await page.getByRole('heading', { name: /login/i }).waitFor()

      await expect(page).toHaveScreenshot(`login-${v.name}.png`, {
        fullPage: true,
        animations: 'disabled',
      })
    } finally {
      await context.close()
    }
  }
})

test('visual snapshots: authenticated routes', async ({ browser }) => {
  const baseURL = getBaseURL()

  const routes = [
    { name: 'recipes', path: '/recipes', heading: /recipes/i },
    { name: 'books', path: '/books', heading: /recipe books/i },
    { name: 'tags', path: '/tags', heading: /tags/i },
    { name: 'settings', path: '/settings', heading: /settings/i },
  ] as const

  for (const v of viewports) {
    const context = await browser.newContext({
      baseURL,
      viewport: v.viewport,
      reducedMotion: 'reduce',
      colorScheme: 'dark',
      locale: 'en-US',
      timezoneId: 'UTC',
      storageState: authStorageStatePath,
    })

    try {
      const page = await context.newPage()
      const mask = [page.getByTestId('app-shell-user')]

      for (const route of routes) {
        await page.goto(route.path, { waitUntil: 'networkidle' })
        await page.getByRole('heading', { name: route.heading }).waitFor()

        await expect(page).toHaveScreenshot(`${route.name}-${v.name}.png`, {
          fullPage: true,
          mask,
          animations: 'disabled',
        })
      }
    } finally {
      await context.close()
    }
  }
})
