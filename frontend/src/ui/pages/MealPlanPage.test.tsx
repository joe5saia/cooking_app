import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { MealPlanPage } from './MealPlanPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <MealPlanPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('MealPlanPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('adds a recipe to the selected day and renders navigation links', async () => {
    const mealPlanEntries: Array<{
      date: string
      recipe: { id: string; title: string }
    }> = []
    const recipes = [
      {
        id: 'r1',
        title: 'Pasta',
        servings: 2,
        prep_time_minutes: 5,
        total_time_minutes: 15,
        source_url: null,
        notes: null,
        recipe_book_id: null,
        tags: [],
        deleted_at: null,
        updated_at: '2025-01-01T00:00:00Z',
      },
    ]

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.includes('/api/v1/recipes') && method === 'GET') {
          return new Response(
            JSON.stringify({
              items: recipes,
              next_cursor: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.includes('/api/v1/meal-plans') && method === 'GET') {
          return new Response(JSON.stringify({ items: mealPlanEntries }), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/meal-plans') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            date?: string
            recipe_id?: string
          }
          const recipe = recipes.find((item) => item.id === body.recipe_id)
          if (!recipe || !body.date) {
            return new Response(null, { status: 400 })
          }
          const entry = {
            date: body.date,
            recipe: { id: recipe.id, title: recipe.title },
          }
          mealPlanEntries.push(entry)
          return new Response(JSON.stringify(entry), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    expect(
      await screen.findByRole('heading', { name: /meal plan/i }),
    ).toBeVisible()

    const calendar = screen.getByRole('grid', { name: /meal plan calendar/i })
    const dayButtons = within(calendar).getAllByRole('button', {
      name: /^select /i,
    })
    await user.click(dayButtons[0])

    const recipeSelect = await screen.findByLabelText(/^recipe$/i)
    await waitFor(() => expect(recipeSelect).not.toBeDisabled())

    await user.selectOptions(recipeSelect, 'r1')
    await user.click(screen.getByRole('button', { name: /add to day/i }))

    const links = await screen.findAllByRole('link', { name: /pasta/i })
    expect(links.length).toBeGreaterThan(0)
  })
})
