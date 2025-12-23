import { expect, test } from '@playwright/test'

import { authStorageStatePath } from './global-setup'

type Rect = { x: number; y: number; width: number; height: number }

const baseURL = process.env.COOKING_APP_BASE_URL ?? 'http://127.0.0.1:5173'

function rectanglesOverlap(a: Rect, b: Rect): boolean {
  return !(
    a.x + a.width <= b.x ||
    b.x + b.width <= a.x ||
    a.y + a.height <= b.y ||
    b.y + b.height <= a.y
  )
}

test('recipe editor: mobile layout avoids action overlap', async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    reducedMotion: 'reduce',
    colorScheme: 'dark',
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    storageState: authStorageStatePath,
  })

  try {
    const page = await context.newPage()
    await page.goto('/recipes/new', { waitUntil: 'networkidle' })
    await page.getByRole('heading', { name: /new recipe/i }).waitFor()

    const stepInstruction = page.getByPlaceholder('Instruction')
    const removeButton = page.getByRole('button', { name: 'Remove' })

    const stepBox = await stepInstruction.boundingBox()
    const removeBox = await removeButton.boundingBox()

    expect(stepBox).not.toBeNull()
    expect(removeBox).not.toBeNull()

    // Keep the remove action outside the step textarea to avoid overlap on small screens.
    const overlap = rectanglesOverlap(stepBox as Rect, removeBox as Rect)
    expect(overlap).toBe(false)
  } finally {
    await context.close()
  }
})

test('recipe list: filters do not overlap', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    reducedMotion: 'reduce',
    colorScheme: 'dark',
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    storageState: authStorageStatePath,
  })

  try {
    const page = await context.newPage()
    await page.goto('/recipes', { waitUntil: 'networkidle' })

    const searchInput = page.getByPlaceholder('Search recipes')
    const bookSelect = page.getByLabel('Filter by book')
    const tagSelect = page.getByLabel('Filter by tag')

    await searchInput.waitFor()
    await bookSelect.waitFor()
    await tagSelect.waitFor()

    const searchBox = await searchInput.boundingBox()
    const bookBox = await bookSelect.boundingBox()
    const tagBox = await tagSelect.boundingBox()

    expect(searchBox).not.toBeNull()
    expect(bookBox).not.toBeNull()
    expect(tagBox).not.toBeNull()

    const searchBookOverlap = rectanglesOverlap(
      searchBox as DOMRect,
      bookBox as DOMRect,
    )
    const bookTagOverlap = rectanglesOverlap(
      bookBox as DOMRect,
      tagBox as DOMRect,
    )

    expect(searchBookOverlap).toBe(false)
    expect(bookTagOverlap).toBe(false)
  } finally {
    await context.close()
  }
})

test('recipe books: create form controls do not overlap', async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    reducedMotion: 'reduce',
    colorScheme: 'dark',
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    storageState: authStorageStatePath,
  })

  try {
    const page = await context.newPage()
    await page.goto('/books', { waitUntil: 'networkidle' })

    const nameInput = page.getByPlaceholder('New book name')
    const addButton = page.getByRole('button', { name: 'Add' })

    await nameInput.waitFor()
    await addButton.waitFor()

    const inputBox = await nameInput.boundingBox()
    const buttonBox = await addButton.boundingBox()

    expect(inputBox).not.toBeNull()
    expect(buttonBox).not.toBeNull()

    const overlap = rectanglesOverlap(inputBox as DOMRect, buttonBox as DOMRect)

    expect(overlap).toBe(false)
  } finally {
    await context.close()
  }
})

test('tags: create form controls do not overlap', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    reducedMotion: 'reduce',
    colorScheme: 'dark',
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    storageState: authStorageStatePath,
  })

  try {
    const page = await context.newPage()
    await page.goto('/tags', { waitUntil: 'networkidle' })

    const nameInput = page.getByPlaceholder('New tag name')
    const addButton = page.getByRole('button', { name: 'Add' })

    await nameInput.waitFor()
    await addButton.waitFor()

    const inputBox = await nameInput.boundingBox()
    const buttonBox = await addButton.boundingBox()

    expect(inputBox).not.toBeNull()
    expect(buttonBox).not.toBeNull()

    const overlap = rectanglesOverlap(inputBox as DOMRect, buttonBox as DOMRect)

    expect(overlap).toBe(false)
  } finally {
    await context.close()
  }
})
