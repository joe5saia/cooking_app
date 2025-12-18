import { Link, useLocation } from 'react-router-dom'

import { Page } from './Page'
import styles from './SettingsPage.module.css'

export function SettingsPage() {
  const location = useLocation()

  return (
    <Page title="Settings">
      <p>Manage access tokens and users.</p>
      <p className={styles.path}>Path: {location.pathname}</p>
      <ul>
        <li>
          <Link to="/settings/tokens">Tokens</Link>
        </li>
        <li>
          <Link to="/settings/users">Users</Link>
        </li>
      </ul>
    </Page>
  )
}
