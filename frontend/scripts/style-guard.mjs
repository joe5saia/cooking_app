/**
 * Style drift guardrails:
 * - disallow JSX inline styles (style={{...}})
 * - disallow raw hex colors in TS/TSX
 * - disallow raw hex colors in *.module.css (use tokens via var(--color-...))
 */

import { spawnSync } from 'node:child_process'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const frontendRoot = path.resolve(__dirname, '..')

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: frontendRoot,
    stdio: 'inherit',
  })
  if (result.error) throw result.error
  return result.status ?? 1
}

async function findFiles(dir, predicate) {
  const entries = await fs.readdir(dir, { withFileTypes: true })
  const out = []
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      out.push(...(await findFiles(fullPath, predicate)))
      continue
    }
    if (entry.isFile() && predicate(fullPath)) out.push(fullPath)
  }
  return out
}

async function checkCSSModulesForHexColors() {
  const hexRegex = /#[0-9a-fA-F]{3,8}\b/g
  const srcDir = path.join(frontendRoot, 'src')

  const files = await findFiles(srcDir, (p) => p.endsWith('.module.css'))
  const violations = []

  for (const filePath of files) {
    const content = await fs.readFile(filePath, 'utf8')
    const lines = content.split('\n')
    for (let i = 0; i < lines.length; i += 1) {
      const line = lines[i]
      const matches = [...line.matchAll(hexRegex)]
      for (const m of matches) {
        violations.push({
          filePath,
          lineNumber: i + 1,
          match: m[0],
        })
      }
    }
  }

  if (violations.length === 0) return 0

  for (const v of violations) {
    const rel = path.relative(frontendRoot, v.filePath)
    console.error(
      `${rel}:${v.lineNumber}: raw hex color "${v.match}" in CSS module`,
    )
  }
  return 1
}

async function main() {
  const astGrepStatus = run('npx', [
    '--no-install',
    'ast-grep',
    'scan',
    '-c',
    'sgconfig.yml',
    'src',
    '--error',
  ])

  const cssStatus = await checkCSSModulesForHexColors()

  const status = astGrepStatus === 0 && cssStatus === 0 ? 0 : 1
  process.exit(status)
}

await main()
