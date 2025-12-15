import { useQuery } from '@tanstack/react-query'
import { NavLink, Outlet, useLocation } from 'react-router-dom'

import styles from './AppShell.module.css'
import { getMe } from '../api/auth'

const navItems = [
  { to: '/recipes', label: 'Recipes' },
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
        <div className={styles.brand}>Cooking App</div>
        <nav className={styles.nav} aria-label="Primary">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `${styles.link} ${isActive ? styles.linkActive : ''}`
              }
              end={item.to === '/recipes'}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className={styles.user}>
          {meQuery.data ? `Signed in as ${meQuery.data.username}` : null}
        </div>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
