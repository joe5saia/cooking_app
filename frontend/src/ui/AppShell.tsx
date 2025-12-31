import { useQuery } from '@tanstack/react-query'
import { NavLink, Outlet, useLocation } from 'react-router-dom'

import styles from './AppShell.module.css'
import { getMe } from '../api/auth'

const navItems = [
  { to: '/recipes', label: 'Recipes' },
  { to: '/meal-plan', label: 'Meal Plan' },
  { to: '/shopping-lists', label: 'Shopping Lists' },
  { to: '/items', label: 'Items' },
  { to: '/books', label: 'Books' },
  { to: '/tags', label: 'Tags' },
  { to: '/settings', label: 'Settings' },
]

export function AppShell() {
  const location = useLocation()
  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    enabled: location.pathname !== '/login' && import.meta.env.MODE !== 'test',
  })

  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <a className={styles.skipLink} href="#main-content">
          Skip to content
        </a>
        <div className={styles.headerInner}>
          <div className={styles.brand}>Cooking App</div>
          <nav className={styles.nav} aria-label="Primary">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `${styles.link} ${isActive ? styles.linkActive : ''}`
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className={styles.user} data-testid="app-shell-user">
            {meQuery.data ? `Signed in as ${meQuery.data.username}` : null}
          </div>
        </div>
      </header>
      <main id="main-content" tabIndex={-1} className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
