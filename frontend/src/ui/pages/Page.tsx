import styles from './Page.module.css'

type PageProps = {
  title: string
  children?: React.ReactNode
}

export function Page({ title, children }: PageProps) {
  return (
    <section className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>{title}</h1>
      </header>
      <div className={styles.body}>{children}</div>
    </section>
  )
}
