import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { ApiError } from '../../api/client'
import { deleteRecipe, getRecipe, restoreRecipe } from '../../api/recipes'
import { listRecipeBooks } from '../../api/recipeBooks'

import styles from './CrudList.module.css'
import { Page } from './Page'

export function RecipeDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState<string | null>(null)

  const recipeQuery = useQuery({
    queryKey: ['recipes', 'detail', id],
    queryFn: () => getRecipe(id ?? ''),
    enabled: Boolean(id),
    retry: false,
  })

  const booksQuery = useQuery({
    queryKey: ['recipe-books'],
    queryFn: listRecipeBooks,
  })

  const deleteMutation = useMutation({
    mutationFn: (recipeID: string) => deleteRecipe(recipeID),
    onMutate: () => {
      setActionError(null)
    },
    onSuccess: async () => {
      setActionError(null)
      await queryClient.invalidateQueries({ queryKey: ['recipes'] })
      navigate('/recipes')
    },
    onError: (e) => {
      setActionError(e instanceof ApiError ? e.message : 'Failed to delete.')
    },
  })

  const restoreMutation = useMutation({
    mutationFn: (recipeID: string) => restoreRecipe(recipeID),
    onMutate: () => {
      setActionError(null)
    },
    onSuccess: async () => {
      setActionError(null)
      await queryClient.invalidateQueries({ queryKey: ['recipes'] })
      await queryClient.invalidateQueries({
        queryKey: ['recipes', 'detail', id],
      })
    },
    onError: (e) => {
      setActionError(e instanceof ApiError ? e.message : 'Failed to restore.')
    },
  })

  const recipe = recipeQuery.data ?? null
  const bookName = useMemo(() => {
    if (!recipe?.recipe_book_id) return null
    const books = booksQuery.data ?? []
    return books.find((b) => b.id === recipe.recipe_book_id)?.name ?? null
  }, [booksQuery.data, recipe])

  if (!id) {
    return (
      <Page title="Recipe">
        <div role="alert" className={styles.alert}>
          Missing recipe id.
        </div>
      </Page>
    )
  }

  return (
    <Page title={recipe?.title ?? 'Recipe'}>
      <div className={styles.section}>
        {actionError ? (
          <div role="alert" className={styles.alert}>
            <div className={styles.row}>
              <div className={styles.grow}>{actionError}</div>
              <button
                className={styles.button}
                type="button"
                onClick={() => setActionError(null)}
              >
                Dismiss
              </button>
            </div>
          </div>
        ) : null}

        {recipeQuery.isPending ? <div>Loading…</div> : null}

        {recipeQuery.isError ? (
          <div role="alert" className={styles.alert}>
            {recipeQuery.error instanceof ApiError &&
            recipeQuery.error.status === 404
              ? 'Recipe not found.'
              : recipeQuery.error instanceof ApiError
                ? recipeQuery.error.message
                : 'Failed to load recipe.'}
          </div>
        ) : null}

        {recipe ? (
          <>
            <div className={styles.row}>
              <Link className={styles.button} to="/recipes">
                Back to recipes
              </Link>
            </div>

            {recipe.deleted_at ? (
              <div role="alert" className={styles.alert}>
                This recipe is deleted.
              </div>
            ) : null}

            <div className={styles.row}>
              {!recipe.deleted_at ? (
                <Link className={styles.button} to={`/recipes/${id}/edit`}>
                  Edit
                </Link>
              ) : null}
              {recipe.deleted_at ? (
                <button
                  className={styles.button}
                  type="button"
                  onClick={() => restoreMutation.mutate(id)}
                  disabled={restoreMutation.isPending}
                >
                  {restoreMutation.isPending ? 'Restoring…' : 'Restore'}
                </button>
              ) : (
                <button
                  className={`${styles.button} ${styles.buttonDanger}`}
                  type="button"
                  onClick={() => {
                    if (!window.confirm('Delete this recipe?')) return
                    deleteMutation.mutate(id)
                  }}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
                </button>
              )}
            </div>

            <div>
              <div>
                <strong>Servings:</strong> {recipe.servings}
              </div>
              <div>
                <strong>Created:</strong> {recipe.created_at}
              </div>
              <div>
                <strong>Updated:</strong> {recipe.updated_at}
              </div>
              {recipe.deleted_at ? (
                <div>
                  <strong>Deleted:</strong> {recipe.deleted_at}
                </div>
              ) : null}
              {bookName ? (
                <div>
                  <strong>Book:</strong>{' '}
                  <Link
                    to={
                      recipe.deleted_at
                        ? `/recipes?book_id=${recipe.recipe_book_id}&include_deleted=true`
                        : `/recipes?book_id=${recipe.recipe_book_id}`
                    }
                  >
                    {bookName}
                  </Link>
                </div>
              ) : null}
              <div>
                <strong>Prep:</strong> {recipe.prep_time_minutes} min
              </div>
              <div>
                <strong>Total:</strong> {recipe.total_time_minutes} min
              </div>
              {recipe.source_url ? (
                <div>
                  <strong>Source:</strong>{' '}
                  <a href={recipe.source_url} target="_blank" rel="noreferrer">
                    {recipe.source_url}
                  </a>
                </div>
              ) : null}
              {recipe.notes ? (
                <div>
                  <strong>Notes:</strong>{' '}
                  <span className={styles.preWrap}>{recipe.notes}</span>
                </div>
              ) : null}
            </div>

            {recipe.tags.length ? (
              <div>
                <strong>Tags:</strong>{' '}
                {recipe.tags.map((t, idx) => (
                  <span key={t.id}>
                    <Link
                      to={
                        recipe.deleted_at
                          ? `/recipes?tag_id=${t.id}&include_deleted=true`
                          : `/recipes?tag_id=${t.id}`
                      }
                    >
                      {t.name}
                    </Link>
                    {idx < recipe.tags.length - 1 ? ', ' : null}
                  </span>
                ))}
              </div>
            ) : null}

            <div>
              <h3>Ingredients</h3>
              {recipe.ingredients.length ? (
                <ul>
                  {recipe.ingredients.map((ing) => (
                    <li key={ing.id}>{formatIngredient(ing)}</li>
                  ))}
                </ul>
              ) : (
                <div>No ingredients.</div>
              )}
            </div>

            <div>
              <h3>Steps</h3>
              {recipe.steps.length ? (
                <ol>
                  {recipe.steps.map((s) => (
                    <li key={s.id}>{s.instruction}</li>
                  ))}
                </ol>
              ) : (
                <div>No steps.</div>
              )}
            </div>
          </>
        ) : null}
      </div>
    </Page>
  )
}

function formatIngredient(ing: {
  quantity: number | null
  quantity_text: string | null
  unit: string | null
  item: string
  prep: string | null
  notes: string | null
  original_text: string | null
}) {
  const pieces: string[] = []
  if (ing.quantity_text?.trim()) pieces.push(ing.quantity_text.trim())
  else if (typeof ing.quantity === 'number') pieces.push(String(ing.quantity))
  if (ing.unit?.trim()) pieces.push(ing.unit.trim())
  pieces.push(ing.item)
  if (ing.prep?.trim()) pieces.push(`(${ing.prep.trim()})`)
  if (ing.notes?.trim()) pieces.push(`— ${ing.notes.trim()}`)
  if (ing.original_text?.trim()) pieces.push(`[${ing.original_text.trim()}]`)
  return pieces.join(' ')
}
