import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'

import { ApiError } from '../../api/client'
import { listRecipeBooks } from '../../api/recipeBooks'
import { listRecipes, restoreRecipe } from '../../api/recipes'
import { listTags } from '../../api/tags'

import { Button, ButtonLink, Card, Input, Select } from '../components'

import styles from './RecipeListPage.module.css'
import { Page } from './Page'

export function RecipeListPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState<string | null>(null)
  const [restoringID, setRestoringID] = useState<string | null>(null)

  const q = searchParams.get('q') ?? ''
  const bookID = searchParams.get('book_id') ?? ''
  const tagID = searchParams.get('tag_id') ?? ''
  const includeDeleted = searchParams.get('include_deleted') === 'true'

  function updateURL(next: {
    q: string
    bookID: string
    tagID: string
    includeDeleted: boolean
  }) {
    const qp = new URLSearchParams()
    if (next.q.trim()) qp.set('q', next.q.trim())
    if (next.bookID) qp.set('book_id', next.bookID)
    if (next.tagID) qp.set('tag_id', next.tagID)
    if (next.includeDeleted) qp.set('include_deleted', 'true')
    setSearchParams(qp, { replace: true })
  }

  const booksQuery = useQuery({
    queryKey: ['recipe-books'],
    queryFn: listRecipeBooks,
  })

  const tagsQuery = useQuery({
    queryKey: ['tags'],
    queryFn: listTags,
  })

  const recipesQuery = useInfiniteQuery({
    queryKey: ['recipes', { q, bookID, tagID, includeDeleted }],
    queryFn: ({ pageParam }) =>
      listRecipes({
        q,
        book_id: bookID || undefined,
        tag_id: tagID || undefined,
        include_deleted: includeDeleted || undefined,
        limit: 25,
        cursor: pageParam ?? undefined,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.next_cursor,
  })

  const recipes = useMemo(
    () => recipesQuery.data?.pages.flatMap((p) => p.items) ?? [],
    [recipesQuery.data],
  )
  const books = useMemo(() => booksQuery.data ?? [], [booksQuery.data])
  const tags = useMemo(() => tagsQuery.data ?? [], [tagsQuery.data])
  const booksByID = useMemo(() => {
    const m = new Map<string, string>()
    for (const b of books) m.set(b.id, b.name)
    return m
  }, [books])

  const restoreMutation = useMutation({
    mutationFn: (recipeID: string) => restoreRecipe(recipeID),
    onMutate: (recipeID) => {
      setActionError(null)
      setRestoringID(recipeID)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['recipes'] })
    },
    onSettled: (_data, _err, recipeID) => {
      setRestoringID((prev) => (prev === recipeID ? null : prev))
    },
    onError: (e) => {
      setActionError(
        e instanceof ApiError ? e.message : 'Failed to restore recipe.',
      )
    },
  })

  return (
    <Page title="Recipes">
      <div className={styles.section}>
        {actionError ? (
          <div role="alert" className={styles.alert}>
            <div className={styles.alertRow}>
              <div className={styles.alertMessage}>{actionError}</div>
              <Button
                size="sm"
                type="button"
                onClick={() => setActionError(null)}
              >
                Dismiss
              </Button>
            </div>
          </div>
        ) : null}

        <div className={styles.row}>
          <ButtonLink size="sm" variant="primary" to="/recipes/new">
            Create recipe
          </ButtonLink>
          {q || bookID || tagID || includeDeleted ? (
            <Button
              size="sm"
              type="button"
              onClick={() =>
                updateURL({
                  q: '',
                  bookID: '',
                  tagID: '',
                  includeDeleted: false,
                })
              }
            >
              Clear filters
            </Button>
          ) : null}
        </div>

        <div className={styles.filters} aria-label="Recipe filters">
          <Input
            value={q}
            onChange={(e) => {
              const next = e.target.value
              updateURL({ q: next, bookID, tagID, includeDeleted })
            }}
            placeholder="Search recipes"
          />

          <Select
            value={bookID}
            onChange={(e) => {
              const next = e.target.value
              updateURL({ q, bookID: next, tagID, includeDeleted })
            }}
            aria-label="Filter by book"
          >
            <option value="">All books</option>
            {books.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>

          <Select
            value={tagID}
            onChange={(e) => {
              const next = e.target.value
              updateURL({ q, bookID, tagID: next, includeDeleted })
            }}
            aria-label="Filter by tag"
          >
            <option value="">All tags</option>
            {tags.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        </div>

        <label className={styles.muted}>
          <input
            type="checkbox"
            checked={includeDeleted}
            onChange={(e) => {
              const next = e.target.checked
              updateURL({ q, bookID, tagID, includeDeleted: next })
            }}
          />{' '}
          Include deleted
        </label>

        {recipesQuery.isPending ? <div>Loading…</div> : null}
        {recipesQuery.isError ? (
          <div role="alert" className={styles.alert}>
            {recipesQuery.error instanceof ApiError
              ? recipesQuery.error.message
              : 'Failed to load recipes.'}
          </div>
        ) : null}

        <div className={styles.list} aria-label="Recipes list">
          {recipes.map((r) => (
            <Card key={r.id} padding="sm" className={styles.item}>
              <div className={styles.itemMain}>
                <div className={styles.row}>
                  <Link to={`/recipes/${r.id}`}>{r.title}</Link>
                  {r.deleted_at ? (
                    <span className={styles.badgeDanger}>Deleted</span>
                  ) : null}
                </div>
                <div className={styles.muted}>
                  Serves {r.servings} • Prep {r.prep_time_minutes} min • Total{' '}
                  {r.total_time_minutes} min • Updated {r.updated_at}
                </div>
                {r.recipe_book_id && booksByID.has(r.recipe_book_id) ? (
                  <div className={styles.muted}>
                    Book:{' '}
                    <Link
                      to={
                        includeDeleted
                          ? `/recipes?book_id=${r.recipe_book_id}&include_deleted=true`
                          : `/recipes?book_id=${r.recipe_book_id}`
                      }
                    >
                      {booksByID.get(r.recipe_book_id)}
                    </Link>
                  </div>
                ) : null}
                {r.tags.length ? (
                  <div className={styles.muted}>
                    {r.tags.map((t, idx) => (
                      <span key={t.id}>
                        <Link
                          to={
                            includeDeleted
                              ? `/recipes?tag_id=${t.id}&include_deleted=true`
                              : `/recipes?tag_id=${t.id}`
                          }
                        >
                          {t.name}
                        </Link>
                        {idx < r.tags.length - 1 ? ', ' : null}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>

              <div className={styles.itemActions}>
                {!r.deleted_at ? (
                  <ButtonLink size="sm" to={`/recipes/${r.id}/edit`}>
                    Edit
                  </ButtonLink>
                ) : null}
                {includeDeleted && r.deleted_at ? (
                  <Button
                    size="sm"
                    type="button"
                    onClick={() => restoreMutation.mutate(r.id)}
                    disabled={restoringID === r.id}
                  >
                    {restoringID === r.id ? 'Restoring…' : 'Restore'}
                  </Button>
                ) : null}
              </div>
            </Card>
          ))}

          {!recipesQuery.isPending && recipes.length === 0 ? (
            <div>No recipes yet.</div>
          ) : null}
        </div>

        {recipesQuery.hasNextPage ? (
          <div className={styles.row}>
            <Button
              size="sm"
              type="button"
              onClick={() => recipesQuery.fetchNextPage()}
              disabled={recipesQuery.isFetchingNextPage}
            >
              {recipesQuery.isFetchingNextPage ? 'Loading…' : 'Load more'}
            </Button>
          </div>
        ) : null}
      </div>
    </Page>
  )
}
