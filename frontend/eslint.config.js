import fs from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import { defineConfig, globalIgnores } from 'eslint/config'

/**
 * typescript-eslint@8 throws on TypeScript 7 (no stable API yet).
 * Prefer a pnpm install of typescript-eslint that is bound to typescript@6.*,
 * which is already present in the local store for side-by-side tooling.
 * @see https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/#running-side-by-side-with-typescript-6.0
 */
async function loadTypescriptEslint() {
  const pnpmDir = path.join(process.cwd(), 'node_modules/.pnpm')
  if (fs.existsSync(pnpmDir)) {
    const match = fs
      .readdirSync(pnpmDir)
      .filter(
        (name) =>
          name.startsWith('typescript-eslint@') &&
          name.includes('__typescript@6.')
      )
      .sort()
      .at(-1)
    if (match) {
      const entry = path.join(
        pnpmDir,
        match,
        'node_modules',
        'typescript-eslint',
        'dist',
        'index.js'
      )
      if (fs.existsSync(entry)) {
        return import(pathToFileURL(entry).href)
      }
    }
  }
  // Fallback (works only when project typescript < 7).
  return import('typescript-eslint')
}

const tseslint = await loadTypescriptEslint()

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    rules: {
      // Compiler skip notices — not actionable without dropping the library/API.
      'react-hooks/incompatible-library': 'off',
      'react-hooks/preserve-manual-memoization': 'off',
      // React Compiler eslint rules that conflict with established app patterns
      // (query→form hydration, datatable refs, shared module co-exports).
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/refs': 'off',
      'react-hooks/use-memo': 'off',
      // Modules routinely co-export hooks/helpers with components.
      'react-refresh/only-export-components': 'off',
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],
    },
  },
])
