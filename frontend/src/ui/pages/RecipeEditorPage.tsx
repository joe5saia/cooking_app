import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { z } from 'zod'

import { ApiError } from '../../api/client'
import { listRecipeBooks } from '../../api/recipeBooks'
import { createRecipe, getRecipe, updateRecipe } from '../../api/recipes'
import { listTags } from '../../api/tags'

import styles from './CrudList.module.css'
import { Page } from './Page'

type RecipeEditorPageProps = {
  mode: 'create' | 'edit'
}

type IngredientFormValues = {
  quantity: string
  quantity_text: string
  unit: string
  item: string
  prep: string
  notes: string
  original_text: string
}

type StepFormValues = {
  instruction: string
}

type RecipeFormValues = {
  title: string
  servings: number
  prep_time_minutes: number
  total_time_minutes: number
  source_url: string
  notes: string
  recipe_book_id: string
  tag_ids: string[]
  ingredients: IngredientFormValues[]
  steps: StepFormValues[]
}

const ingredientSchema = z.object({
  quantity: z
    .string()
    .trim()
    .refine(
      (v) =>
        v === '' || (!Number.isNaN(Number(v)) && Number.isFinite(Number(v))),
      'Quantity must be a number',
    ),
  quantity_text: z.string(),
  unit: z.string(),
  item: z.string().trim().min(1, 'Item is required'),
  prep: z.string(),
  notes: z.string(),
  original_text: z.string(),
})

const stepSchema = z.object({
  instruction: z.string().trim().min(1, 'Instruction is required'),
})

const recipeSchema = z.object({
  title: z.string().trim().min(1, 'Title is required'),
  servings: z.number().int().finite().gt(0, 'Servings must be > 0'),
  prep_time_minutes: z.number().int().finite().min(0, 'Prep time must be >= 0'),
  total_time_minutes: z
    .number()
    .int()
    .finite()
    .min(0, 'Total time must be >= 0'),
  source_url: z
    .string()
    .trim()
    .refine(
      (v) => v === '' || z.string().url().safeParse(v).success,
      'Source URL must be a valid URL',
    ),
  notes: z.string(),
  recipe_book_id: z.string(),
  tag_ids: z.array(z.string()),
  ingredients: z.array(ingredientSchema),
  steps: z.array(stepSchema).min(1, 'At least one step is required'),
})

export function RecipeEditorPage({ mode }: RecipeEditorPageProps) {
  const { id } = useParams()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  const booksQuery = useQuery({
    queryKey: ['recipe-books'],
    queryFn: listRecipeBooks,
  })

  const tagsQuery = useQuery({
    queryKey: ['tags'],
    queryFn: listTags,
  })

  const recipeQuery = useQuery({
    queryKey: ['recipes', 'detail', id],
    queryFn: () => getRecipe(id ?? ''),
    enabled: mode === 'edit' && Boolean(id),
    retry: false,
  })

  const form = useForm<RecipeFormValues>({
    resolver: zodResolver(recipeSchema),
    defaultValues: {
      title: '',
      servings: 1,
      prep_time_minutes: 0,
      total_time_minutes: 0,
      source_url: '',
      notes: '',
      recipe_book_id: '',
      tag_ids: [],
      ingredients: [],
      steps: [{ instruction: '' }],
    },
  })

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = form

  const ingredients = useFieldArray({
    control: form.control,
    name: 'ingredients',
  })
  const steps = useFieldArray({ control: form.control, name: 'steps' })

  const createMutation = useMutation({
    mutationFn: createRecipe,
    onSuccess: (created) => {
      navigate(`/recipes/${created.id}`)
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to create recipe.')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (params: {
      id: string
      body: ReturnType<typeof buildUpsert>
    }) => updateRecipe(params.id, params.body),
    onSuccess: (updated) => {
      navigate(`/recipes/${updated.id}`)
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to update recipe.')
    },
  })

  const books = useMemo(() => booksQuery.data ?? [], [booksQuery.data])
  const tags = useMemo(() => tagsQuery.data ?? [], [tagsQuery.data])

  useEffect(() => {
    if (mode !== 'edit') return
    const r = recipeQuery.data
    if (!r) return

    reset({
      title: r.title,
      servings: r.servings,
      prep_time_minutes: r.prep_time_minutes,
      total_time_minutes: r.total_time_minutes,
      source_url: r.source_url ?? '',
      notes: r.notes ?? '',
      recipe_book_id: r.recipe_book_id ?? '',
      tag_ids: r.tags.map((t) => t.id),
      ingredients: r.ingredients
        .sort((a, b) => a.position - b.position)
        .map((ing) => ({
          quantity: ing.quantity === null ? '' : String(ing.quantity),
          quantity_text: ing.quantity_text ?? '',
          unit: ing.unit ?? '',
          item: ing.item,
          prep: ing.prep ?? '',
          notes: ing.notes ?? '',
          original_text: ing.original_text ?? '',
        })),
      steps:
        r.steps.length > 0
          ? r.steps
              .sort((a, b) => a.step_number - b.step_number)
              .map((s) => ({ instruction: s.instruction }))
          : [{ instruction: '' }],
    })
  }, [mode, recipeQuery.data, reset])

  const title = mode === 'create' ? 'New Recipe' : 'Edit Recipe'
  const isDeleted = mode === 'edit' && Boolean(recipeQuery.data?.deleted_at)

  if (mode === 'edit' && !id) {
    return (
      <Page title={title}>
        <div role="alert" className={styles.alert}>
          Missing recipe id.
        </div>
      </Page>
    )
  }

  const onSubmit = handleSubmit((values) => {
    setError(null)
    if (isDeleted) {
      setError('This recipe is deleted. Restore it before editing.')
      return
    }
    const body = buildUpsert(values)
    if (mode === 'create') {
      createMutation.mutate(body)
    } else {
      updateMutation.mutate({ id: id ?? '', body })
    }
  })

  return (
    <Page title={title}>
      <div className={styles.section}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        {isDeleted ? (
          <div role="alert" className={styles.alert}>
            This recipe is deleted. Restore it before editing.{' '}
            <Link to={`/recipes/${id}`}>Go to recipe</Link>
          </div>
        ) : null}

        {mode === 'edit' && recipeQuery.isPending ? <div>Loading…</div> : null}
        {mode === 'edit' && recipeQuery.isError ? (
          <div role="alert" className={styles.alert}>
            {recipeQuery.error instanceof ApiError &&
            recipeQuery.error.status === 404
              ? 'Recipe not found.'
              : 'Failed to load recipe.'}
          </div>
        ) : null}

        <form onSubmit={onSubmit} aria-label="Recipe editor">
          <div className={styles.section}>
            <label>
              Title
              <input className={styles.input} {...register('title')} />
              {errors.title ? (
                <div role="alert" className={styles.alert}>
                  {errors.title.message}
                </div>
              ) : null}
            </label>

            <div className={styles.filters}>
              <label>
                Servings
                <input
                  className={styles.input}
                  type="number"
                  inputMode="numeric"
                  {...register('servings', { valueAsNumber: true })}
                />
                {errors.servings ? (
                  <div role="alert" className={styles.alert}>
                    {errors.servings.message}
                  </div>
                ) : null}
              </label>
              <label>
                Prep (min)
                <input
                  className={styles.input}
                  type="number"
                  inputMode="numeric"
                  {...register('prep_time_minutes', { valueAsNumber: true })}
                />
                {errors.prep_time_minutes ? (
                  <div role="alert" className={styles.alert}>
                    {errors.prep_time_minutes.message}
                  </div>
                ) : null}
              </label>
              <label>
                Total (min)
                <input
                  className={styles.input}
                  type="number"
                  inputMode="numeric"
                  {...register('total_time_minutes', { valueAsNumber: true })}
                />
                {errors.total_time_minutes ? (
                  <div role="alert" className={styles.alert}>
                    {errors.total_time_minutes.message}
                  </div>
                ) : null}
              </label>
            </div>

            <label>
              Source URL
              <input
                className={styles.input}
                placeholder="https://example.com"
                {...register('source_url')}
              />
              {errors.source_url ? (
                <div role="alert" className={styles.alert}>
                  {errors.source_url.message}
                </div>
              ) : null}
            </label>

            <label>
              Notes
              <textarea className={styles.input} {...register('notes')} />
            </label>

            <label>
              Recipe book
              <select className={styles.input} {...register('recipe_book_id')}>
                <option value="">None</option>
                {books.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>

            <div>
              <div>
                <strong>Tags</strong>
              </div>
              <div className={styles.row}>
                {tags.map((t) => (
                  <label key={t.id}>
                    <input
                      type="checkbox"
                      value={t.id}
                      {...register('tag_ids')}
                    />{' '}
                    {t.name}
                  </label>
                ))}
                {!tags.length ? (
                  <div className={styles.muted}>No tags yet.</div>
                ) : null}
              </div>
            </div>

            <div>
              <div className={styles.row}>
                <h3 className={styles.heading}>Ingredients</h3>
                <button
                  className={styles.button}
                  type="button"
                  onClick={() =>
                    ingredients.append({
                      quantity: '',
                      quantity_text: '',
                      unit: '',
                      item: '',
                      prep: '',
                      notes: '',
                      original_text: '',
                    })
                  }
                >
                  Add ingredient
                </button>
              </div>

              {ingredients.fields.map((field, index) => (
                <div key={field.id} className={styles.item}>
                  <div className={styles.section}>
                    <div className={styles.filters}>
                      <input
                        className={styles.input}
                        placeholder="Qty"
                        {...register(`ingredients.${index}.quantity`)}
                      />
                      <input
                        className={styles.input}
                        placeholder="Qty text"
                        {...register(`ingredients.${index}.quantity_text`)}
                      />
                      <input
                        className={styles.input}
                        placeholder="Unit"
                        {...register(`ingredients.${index}.unit`)}
                      />
                    </div>
                    <input
                      className={styles.input}
                      placeholder="Item"
                      {...register(`ingredients.${index}.item`)}
                    />
                    {errors.ingredients?.[index]?.item ? (
                      <div role="alert" className={styles.alert}>
                        {errors.ingredients[index]?.item?.message}
                      </div>
                    ) : null}
                    {errors.ingredients?.[index]?.quantity ? (
                      <div role="alert" className={styles.alert}>
                        {errors.ingredients[index]?.quantity?.message}
                      </div>
                    ) : null}
                    <input
                      className={styles.input}
                      placeholder="Prep"
                      {...register(`ingredients.${index}.prep`)}
                    />
                    <input
                      className={styles.input}
                      placeholder="Ingredient notes"
                      {...register(`ingredients.${index}.notes`)}
                    />
                    <input
                      className={styles.input}
                      placeholder="Original text"
                      {...register(`ingredients.${index}.original_text`)}
                    />
                  </div>
                  <div className={styles.actions}>
                    <button
                      className={`${styles.button} ${styles.buttonDanger}`}
                      type="button"
                      onClick={() => ingredients.remove(index)}
                    >
                      Remove
                    </button>
                  </div>
                </div>
              ))}
            </div>

            <div>
              <div className={styles.row}>
                <h3 className={styles.heading}>Steps</h3>
                <button
                  className={styles.button}
                  type="button"
                  onClick={() => steps.append({ instruction: '' })}
                >
                  Add step
                </button>
              </div>
              {errors.steps ? (
                <div role="alert" className={styles.alert}>
                  {errors.steps.message}
                </div>
              ) : null}

              {steps.fields.map((field, index) => (
                <div key={field.id} className={styles.item}>
                  <div className={styles.section}>
                    <div className={styles.muted}>Step {index + 1}</div>
                    <textarea
                      className={styles.input}
                      placeholder="Instruction"
                      {...register(`steps.${index}.instruction`)}
                    />
                    {errors.steps?.[index]?.instruction ? (
                      <div role="alert" className={styles.alert}>
                        {errors.steps[index]?.instruction?.message}
                      </div>
                    ) : null}
                  </div>
                  <div className={styles.actions}>
                    <button
                      className={`${styles.button} ${styles.buttonDanger}`}
                      type="button"
                      onClick={() => steps.remove(index)}
                      disabled={steps.fields.length <= 1}
                    >
                      Remove
                    </button>
                  </div>
                </div>
              ))}
            </div>

            <div className={styles.row}>
              <button
                className={styles.button}
                type="submit"
                disabled={
                  isSubmitting ||
                  createMutation.isPending ||
                  updateMutation.isPending ||
                  isDeleted
                }
              >
                {mode === 'create' ? 'Create' : 'Save'}
              </button>
              <Link
                className={styles.button}
                to={mode === 'create' ? '/recipes' : `/recipes/${id}`}
              >
                Cancel
              </Link>
            </div>
          </div>
        </form>
      </div>
    </Page>
  )
}

function buildUpsert(values: RecipeFormValues) {
  const trimmedTitle = values.title.trim()
  const sourceURL = values.source_url.trim()
  const notes = values.notes.trim()

  return {
    title: trimmedTitle,
    servings: values.servings,
    prep_time_minutes: values.prep_time_minutes,
    total_time_minutes: values.total_time_minutes,
    source_url: sourceURL === '' ? null : sourceURL,
    notes: notes === '' ? null : notes,
    recipe_book_id: values.recipe_book_id === '' ? null : values.recipe_book_id,
    tag_ids: values.tag_ids,
    ingredients: values.ingredients.map((ing, index) => ({
      position: index + 1,
      quantity: ing.quantity.trim() === '' ? null : Number(ing.quantity),
      quantity_text:
        ing.quantity_text.trim() === '' ? null : ing.quantity_text.trim(),
      unit: ing.unit.trim() === '' ? null : ing.unit.trim(),
      item: ing.item.trim(),
      prep: ing.prep.trim() === '' ? null : ing.prep.trim(),
      notes: ing.notes.trim() === '' ? null : ing.notes.trim(),
      original_text:
        ing.original_text.trim() === '' ? null : ing.original_text.trim(),
    })),
    steps: values.steps.map((s, index) => ({
      step_number: index + 1,
      instruction: s.instruction.trim(),
    })),
  }
}
