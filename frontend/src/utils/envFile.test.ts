import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { afterEach, expect, it } from 'vitest'

import { loadEnvFile } from './envFile'

const envKeys = ['COOKING_APP_TEST_ONE', 'COOKING_APP_TEST_TWO']

afterEach(() => {
  for (const key of envKeys) {
    delete process.env[key]
  }
})

it('loads env values from file when not already set', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'cooking-app-env-'))
  const filePath = path.join(dir, '.env.test')
  await fs.writeFile(
    filePath,
    ['COOKING_APP_TEST_ONE=hello', 'COOKING_APP_TEST_TWO="world"'].join('\n'),
  )

  loadEnvFile({ envFilePath: filePath })

  expect(process.env.COOKING_APP_TEST_ONE).toBe('hello')
  expect(process.env.COOKING_APP_TEST_TWO).toBe('world')
})

it('does not override existing values by default', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'cooking-app-env-'))
  const filePath = path.join(dir, '.env.test')
  await fs.writeFile(filePath, 'COOKING_APP_TEST_ONE=new')

  process.env.COOKING_APP_TEST_ONE = 'existing'
  loadEnvFile({ envFilePath: filePath })

  expect(process.env.COOKING_APP_TEST_ONE).toBe('existing')
})
