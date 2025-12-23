import fs from 'node:fs'

type LoadEnvFileOptions = {
  envFilePath: string
  override?: boolean
}

function stripQuotes(value: string): string {
  if (
    (value.startsWith('"') && value.endsWith('"')) ||
    (value.startsWith("'") && value.endsWith("'"))
  ) {
    return value.slice(1, -1)
  }
  return value
}

/**
 * Load a simple KEY=VALUE env file into process.env for Node-only tooling.
 */
export function loadEnvFile({
  envFilePath,
  override = false,
}: LoadEnvFileOptions): void {
  if (!fs.existsSync(envFilePath)) {
    return
  }

  const content = fs.readFileSync(envFilePath, 'utf-8')
  const lines = content.split(/\r?\n/)

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) {
      continue
    }

    const separatorIndex = trimmed.indexOf('=')
    if (separatorIndex <= 0) {
      continue
    }

    const key = trimmed.slice(0, separatorIndex).trim()
    if (!key) {
      continue
    }

    const rawValue = trimmed.slice(separatorIndex + 1).trim()
    const value = stripQuotes(rawValue)

    if (!override && process.env[key] !== undefined) {
      continue
    }

    process.env[key] = value
  }
}
