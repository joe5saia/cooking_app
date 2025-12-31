import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'

import { ApiError } from '../../api/client'
import {
  createMealPlan,
  deleteMealPlan,
  listMealPlans,
  type MealPlanEntry,
} from '../../api/mealPlans'
import { listRecipes } from '../../api/recipes'
import {
  addShoppingListItemsFromMealPlan,
  listShoppingLists,
} from '../../api/shoppingLists'
import { Button, Card, FormField, Input, Select } from '../components'
import { Page } from './Page'
import styles from './MealPlanPage.module.css'

const weekdayLabels = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
const calendarWeeks = 6

type CalendarDay = {
  date: Date
  key: string
  label: string
  isCurrentMonth: boolean
  isToday: boolean
}

/** Format a local date as YYYY-MM-DD for stable day keys. */
function formatDateKey(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/** Parse a YYYY-MM-DD key into a local Date instance. */
function parseDateKey(key: string): Date {
  const [year, month, day] = key.split('-').map((value) => Number(value))
  return new Date(year, month - 1, day)
}

type MealPlanState = Record<string, MealPlanEntry[]>

/** Build a 6-week calendar grid for the current month view. */
function buildMonthGrid(viewDate: Date, todayKey: string): CalendarDay[] {
  const year = viewDate.getFullYear()
  const month = viewDate.getMonth()
  const firstOfMonth = new Date(year, month, 1)
  const startOffset = (firstOfMonth.getDay() + 6) % 7
  const gridStart = new Date(year, month, 1 - startOffset)

  const days: CalendarDay[] = []
  for (let i = 0; i < calendarWeeks * 7; i += 1) {
    const date = new Date(gridStart)
    date.setDate(gridStart.getDate() + i)
    const key = formatDateKey(date)
    days.push({
      date,
      key,
      label: String(date.getDate()),
      isCurrentMonth: date.getMonth() === month,
      isToday: key === todayKey,
    })
  }

  return days
}

/** Meal planning calendar with inline recipe navigation and selection. */
export function MealPlanPage() {
  const todayKey = useMemo(() => formatDateKey(new Date()), [])
  const [viewDate, setViewDate] = useState(
    () => new Date(new Date().getFullYear(), new Date().getMonth(), 1),
  )
  // Calendar grid vs. day detail view.
  const [activeView, setActiveView] = useState<'calendar' | 'day'>('calendar')
  const [selectedDateKey, setSelectedDateKey] = useState(todayKey)
  const [selectedRecipeId, setSelectedRecipeId] = useState('')
  const [recipeFilter, setRecipeFilter] = useState('')
  const [notice, setNotice] = useState<string | null>(null)
  const [shoppingListNotice, setShoppingListNotice] = useState<string | null>(
    null,
  )
  const [selectedShoppingListID, setSelectedShoppingListID] = useState('')
  const queryClient = useQueryClient()

  const recipesQuery = useQuery({
    queryKey: ['recipes', 'meal-plan'],
    queryFn: () => listRecipes({ limit: 250 }),
  })

  const recipes = useMemo(
    () => (recipesQuery.data?.items ?? []).filter((item) => !item.deleted_at),
    [recipesQuery.data],
  )

  const filteredRecipes = useMemo(() => {
    const needle = recipeFilter.trim().toLowerCase()
    if (!needle) {
      return recipes
    }
    return recipes.filter((recipe) =>
      recipe.title.toLowerCase().includes(needle),
    )
  }, [recipes, recipeFilter])

  const calendarDays = useMemo(
    () => buildMonthGrid(viewDate, todayKey),
    [viewDate, todayKey],
  )

  const calendarRange = useMemo(() => {
    if (calendarDays.length === 0) {
      return { start: todayKey, end: todayKey }
    }
    const start = formatDateKey(calendarDays[0].date)
    const end = formatDateKey(calendarDays[calendarDays.length - 1].date)
    return { start, end }
  }, [calendarDays, todayKey])

  const mealPlansQuery = useQuery({
    queryKey: ['meal-plans', calendarRange.start, calendarRange.end],
    queryFn: () =>
      listMealPlans({
        start: calendarRange.start,
        end: calendarRange.end,
      }),
  })

  const shoppingListsQuery = useQuery({
    queryKey: ['shopping-lists', 'meal-plan', selectedDateKey],
    queryFn: () =>
      listShoppingLists({ start: selectedDateKey, end: selectedDateKey }),
    enabled: activeView === 'day',
  })

  const mealPlan = useMemo(() => {
    const items = mealPlansQuery.data?.items ?? []
    const grouped: MealPlanState = {}
    for (const entry of items) {
      if (!grouped[entry.date]) {
        grouped[entry.date] = []
      }
      grouped[entry.date].push(entry)
    }
    return grouped
  }, [mealPlansQuery.data])

  const selectedDate = useMemo(
    () => parseDateKey(selectedDateKey),
    [selectedDateKey],
  )
  const selectedEntries = mealPlan[selectedDateKey] ?? []
  const monthLabel = useMemo(
    () =>
      viewDate.toLocaleDateString('en-US', {
        month: 'long',
        year: 'numeric',
      }),
    [viewDate],
  )
  const selectedLabel = useMemo(
    () =>
      selectedDate.toLocaleDateString('en-US', {
        weekday: 'long',
        month: 'long',
        day: 'numeric',
      }),
    [selectedDate],
  )

  const addMealPlanMutation = useMutation({
    mutationFn: (params: { date: string; recipeID: string }) =>
      createMealPlan({ date: params.date, recipe_id: params.recipeID }),
    onSuccess: async () => {
      setSelectedRecipeId('')
      setNotice(null)
      await queryClient.invalidateQueries({ queryKey: ['meal-plans'] })
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) {
        setNotice('Already planned for this day.')
        return
      }
      setNotice('Unable to add recipe to the meal plan.')
    },
  })

  const deleteMealPlanMutation = useMutation({
    mutationFn: (params: { date: string; recipeID: string }) =>
      deleteMealPlan({ date: params.date, recipe_id: params.recipeID }),
    onSuccess: async () => {
      setNotice(null)
      await queryClient.invalidateQueries({ queryKey: ['meal-plans'] })
    },
    onError: () => {
      setNotice('Unable to remove recipe from the meal plan.')
    },
  })

  const addToShoppingListMutation = useMutation({
    mutationFn: (params: { listID: string; date: string }) =>
      addShoppingListItemsFromMealPlan(params.listID, params.date),
    onSuccess: () => {
      setShoppingListNotice('Added day ingredients to the shopping list.')
    },
    onError: (err) => {
      setShoppingListNotice(
        err instanceof ApiError
          ? err.message
          : 'Unable to add items to the shopping list.',
      )
    },
  })

  /** Move the month view forward or backward. */
  function shiftMonth(delta: number) {
    setViewDate(
      (prev) => new Date(prev.getFullYear(), prev.getMonth() + delta, 1),
    )
  }

  /** Open a day detail view from the calendar. */
  function handleOpenDay(dayKey: string) {
    setSelectedDateKey(dayKey)
    setSelectedRecipeId('')
    setSelectedShoppingListID('')
    setShoppingListNotice(null)
    setActiveView('day')
    setNotice(null)
  }

  /** Jump both view and selection back to today. */
  function handleJumpToToday() {
    const today = new Date()
    setViewDate(new Date(today.getFullYear(), today.getMonth(), 1))
    setSelectedDateKey(formatDateKey(today))
    setNotice(null)
  }

  /** Return to the month grid view. */
  function handleBackToCalendar() {
    setActiveView('calendar')
    setNotice(null)
    setShoppingListNotice(null)
  }

  /** Add the currently selected recipe to the chosen day. */
  function handleAddRecipe() {
    if (!selectedRecipeId) {
      setNotice('Select a recipe to add.')
      return
    }

    if (selectedEntries.some((entry) => entry.recipe.id === selectedRecipeId)) {
      setNotice('Already planned for this day.')
      return
    }

    addMealPlanMutation.mutate({
      date: selectedDateKey,
      recipeID: selectedRecipeId,
    })
  }

  /** Remove a recipe from the selected day. */
  function handleRemoveRecipe(recipeId: string) {
    deleteMealPlanMutation.mutate({ date: selectedDateKey, recipeID: recipeId })
  }

  function handleAddMealPlanToList() {
    if (!selectedShoppingListID) {
      setShoppingListNotice('Select a shopping list.')
      return
    }
    addToShoppingListMutation.mutate({
      listID: selectedShoppingListID,
      date: selectedDateKey,
    })
  }

  /** Render a day-level recipe link chip. */
  function renderRecipeChip(entry: MealPlanEntry) {
    return (
      <Link
        key={entry.recipe.id}
        to={`/recipes/${entry.recipe.id}`}
        className={styles.recipeChip}
      >
        {entry.recipe.title}
      </Link>
    )
  }

  return (
    <Page title="Meal Plan">
      <div className={styles.page}>
        {activeView === 'calendar' ? (
          <Card className={styles.calendarCard}>
            <div className={styles.calendarHeader}>
              <div>
                <div className={styles.kicker}>Plan the week</div>
                <div className={styles.subtitle}>
                  Pick a day to review and add recipes.
                </div>
              </div>
              <div className={styles.controls}>
                <Button size="sm" type="button" onClick={() => shiftMonth(-1)}>
                  Previous
                </Button>
                <div className={styles.monthLabel}>{monthLabel}</div>
                <Button size="sm" type="button" onClick={() => shiftMonth(1)}>
                  Next
                </Button>
                <Button size="sm" type="button" onClick={handleJumpToToday}>
                  Today
                </Button>
              </div>
            </div>

            <div className={styles.calendarScroller}>
              <div className={styles.weekdays} aria-hidden="true">
                {weekdayLabels.map((label) => (
                  <div key={label} className={styles.weekday}>
                    {label}
                  </div>
                ))}
              </div>

              <div
                className={styles.calendarGrid}
                role="grid"
                aria-label="Meal plan calendar"
              >
                {calendarDays.map((day) => {
                  const entries = mealPlan[day.key] ?? []
                  const extraCount = entries.length - 3
                  const isSelected = day.key === selectedDateKey
                  return (
                    <div
                      key={day.key}
                      role="gridcell"
                      className={styles.dayCell}
                      data-selected={isSelected}
                      data-today={day.isToday}
                      data-outside={!day.isCurrentMonth}
                    >
                      <div className={styles.dayHeader}>
                        <button
                          type="button"
                          className={styles.daySelect}
                          aria-pressed={isSelected}
                          aria-label={`Select ${day.date.toLocaleDateString(
                            'en-US',
                            {
                              weekday: 'long',
                              month: 'long',
                              day: 'numeric',
                            },
                          )}`}
                          onClick={() => handleOpenDay(day.key)}
                        >
                          <span className={styles.dayNumber}>{day.label}</span>
                        </button>
                        <Button
                          size="sm"
                          variant="ghost"
                          type="button"
                          className={styles.dayAction}
                          onClick={() => handleOpenDay(day.key)}
                        >
                          Add
                        </Button>
                      </div>
                      <div className={styles.dayRecipes}>
                        {entries.slice(0, 3).map(renderRecipeChip)}
                        {extraCount > 0 ? (
                          <div className={styles.moreCount}>
                            +{extraCount} more
                          </div>
                        ) : null}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          </Card>
        ) : (
          <div className={styles.dayView}>
            <div className={styles.dayToolbar}>
              <Button
                size="sm"
                variant="ghost"
                type="button"
                onClick={handleBackToCalendar}
              >
                Back to calendar
              </Button>
              <Button size="sm" type="button" onClick={handleJumpToToday}>
                Today
              </Button>
            </div>

            <Card className={styles.detailCard}>
              <div className={styles.detailHeader}>
                <div>
                  <div className={styles.detailLabel}>Selected day</div>
                  <div className={styles.detailTitle}>{selectedLabel}</div>
                  <div className={styles.detailMeta}>
                    {selectedEntries.length === 0
                      ? 'No recipes yet'
                      : `${selectedEntries.length} recipe${
                          selectedEntries.length === 1 ? '' : 's'
                        } planned`}
                  </div>
                </div>
              </div>

              <div className={styles.planList} aria-live="polite">
                {selectedEntries.length === 0 ? (
                  <div className={styles.emptyState}>
                    Pick a recipe below to start building the day.
                  </div>
                ) : (
                  selectedEntries.map((entry) => (
                    <div key={entry.recipe.id} className={styles.planItem}>
                      <Link
                        to={`/recipes/${entry.recipe.id}`}
                        className={styles.planLink}
                      >
                        {entry.recipe.title}
                      </Link>
                      <Button
                        size="sm"
                        variant="ghost"
                        type="button"
                        onClick={() => handleRemoveRecipe(entry.recipe.id)}
                        disabled={deleteMealPlanMutation.isPending}
                      >
                        Remove
                      </Button>
                    </div>
                  ))
                )}
              </div>

              <div className={styles.addForm}>
                <FormField label="Find a recipe">
                  {({ id, describedBy }) => (
                    <Input
                      id={id}
                      aria-describedby={describedBy}
                      value={recipeFilter}
                      onChange={(event) => setRecipeFilter(event.target.value)}
                      placeholder="Filter by title"
                    />
                  )}
                </FormField>

                <FormField label="Recipe">
                  {({ id, describedBy }) => (
                    <Select
                      id={id}
                      aria-describedby={describedBy}
                      value={selectedRecipeId}
                      onChange={(event) =>
                        setSelectedRecipeId(event.target.value)
                      }
                      disabled={recipesQuery.isPending || recipes.length === 0}
                    >
                      <option value="">Select a recipe</option>
                      {filteredRecipes.map((recipe) => (
                        <option key={recipe.id} value={recipe.id}>
                          {recipe.title}
                        </option>
                      ))}
                    </Select>
                  )}
                </FormField>

                <Button
                  variant="primary"
                  type="button"
                  onClick={handleAddRecipe}
                  disabled={
                    recipesQuery.isPending ||
                    recipes.length === 0 ||
                    addMealPlanMutation.isPending
                  }
                >
                  {addMealPlanMutation.isPending ? 'Adding…' : 'Add to day'}
                </Button>
                {notice ? <div className={styles.notice}>{notice}</div> : null}
                {recipesQuery.isError ? (
                  <div className={styles.notice}>
                    Unable to load recipes for planning.
                  </div>
                ) : null}
                {mealPlansQuery.isError ? (
                  <div className={styles.notice}>
                    Unable to load meal plan entries.
                  </div>
                ) : null}
              </div>

              <div className={styles.shoppingListPanel}>
                <FormField label="Shopping list for this day">
                  {({ id, describedBy }) => (
                    <Select
                      id={id}
                      aria-describedby={describedBy}
                      value={selectedShoppingListID}
                      onChange={(event) => {
                        setSelectedShoppingListID(event.target.value)
                        setShoppingListNotice(null)
                      }}
                      disabled={shoppingListsQuery.isPending}
                    >
                      <option value="">Select a list</option>
                      {(shoppingListsQuery.data ?? []).map((list) => (
                        <option key={list.id} value={list.id}>
                          {list.name} • {list.list_date}
                        </option>
                      ))}
                    </Select>
                  )}
                </FormField>
                <div className={styles.shoppingListActions}>
                  <Button
                    variant="primary"
                    type="button"
                    onClick={handleAddMealPlanToList}
                    disabled={addToShoppingListMutation.isPending}
                  >
                    {addToShoppingListMutation.isPending
                      ? 'Adding…'
                      : 'Add day to list'}
                  </Button>
                  <Link
                    className={styles.shoppingListLink}
                    to="/shopping-lists"
                  >
                    Manage lists
                  </Link>
                </div>
                {shoppingListNotice ? (
                  <div className={styles.notice}>{shoppingListNotice}</div>
                ) : null}
                {shoppingListsQuery.isError ? (
                  <div className={styles.notice}>
                    Unable to load shopping lists.
                  </div>
                ) : null}
                {!shoppingListsQuery.isPending &&
                (shoppingListsQuery.data ?? []).length === 0 ? (
                  <div className={styles.notice}>No lists for this date.</div>
                ) : null}
              </div>
            </Card>
          </div>
        )}
      </div>
    </Page>
  )
}
