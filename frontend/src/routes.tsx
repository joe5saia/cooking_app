import { Route, Routes } from 'react-router-dom'

import { RequireAuth } from './auth/RequireAuth'
import { AppShell } from './ui/AppShell'
import { BookListPage } from './ui/pages/BookListPage'
import { LoginPage } from './ui/pages/LoginPage'
import { RecipeDetailPage } from './ui/pages/RecipeDetailPage'
import { RecipeEditorPage } from './ui/pages/RecipeEditorPage'
import { RecipeListPage } from './ui/pages/RecipeListPage'
import { SettingsPage } from './ui/pages/SettingsPage'
import { TagListPage } from './ui/pages/TagListPage'
import { TokenManagerPage } from './ui/pages/TokenManagerPage'
import { UserManagerPage } from './ui/pages/UserManagerPage'

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppShell />}>
          <Route path="/" element={<RecipeListPage />} />
          <Route path="/recipes" element={<RecipeListPage />} />
          <Route
            path="/recipes/new"
            element={<RecipeEditorPage mode="create" />}
          />
          <Route path="/recipes/:id" element={<RecipeDetailPage />} />
          <Route
            path="/recipes/:id/edit"
            element={<RecipeEditorPage mode="edit" />}
          />

          <Route path="/books" element={<BookListPage />} />
          <Route path="/tags" element={<TagListPage />} />

          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/settings/tokens" element={<TokenManagerPage />} />
          <Route path="/settings/users" element={<UserManagerPage />} />
        </Route>
      </Route>
    </Routes>
  )
}
