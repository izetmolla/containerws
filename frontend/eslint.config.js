import { createRequire } from 'node:module'
import fs from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import { defineConfig, globalIgnores } from 'eslint/config'

/**
 * typescript-eslint@8 does not support TypeScript 7 yet and throws on import when
 * `require("typescript")` resolves to 7.x. Load it against TypeScript 6 instead:
 * 1) Prefer a pnpm store build already bound to typescript@6.*
 * 2) Else redirect Node's "typescript" resolution to the `typescript6` alias
 *    (npm:typescript@6) before importing typescript-eslint.
 *
 * @see https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/#running-side-by-side-with-typescript-6.0
 * @see https://github.com/typescript-eslint/typescript-eslint/issues/10940
 */
async function loadTypescriptEslint() {
  const require = createRequire(import.meta.url)
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

  // Fresh CI installs: only TS7 is linked. Redirect "typescript" → typescript6.
  let ts6Main
  try {
    ts6Main = require.resolve('typescript6')
  } catch {
    throw new Error(
      'typescript-eslint needs TypeScript 6 for ESLint under TS7. ' +
        'Install the alias: pnpm add -D typescript6@npm:typescript@6.0.3'
    )
  }

  const Module = require('module')
  const originalResolveFilename = Module._resolveFilename
  Module._resolveFilename = function (request, parent, isMain, options) {
    if (request === 'typescript') {
      return ts6Main
    }
    if (typeof request === 'string' && request.startsWith('typescript/')) {
      const sub = request.slice('typescript/'.length)
      return originalResolveFilename.call(
        this,
        `typescript6/${sub}`,
        parent,
        isMain,
        options
      )
    }
    return originalResolveFilename.call(this, request, parent, isMain, options)
  }

  try {
    // Clear any cached typescript / typescript-eslint loaded against TS7.
    for (const key of Object.keys(require.cache)) {
      if (
        key.includes(`${path.sep}typescript${path.sep}`) ||
        key.includes(`${path.sep}typescript-eslint${path.sep}`) ||
        key.includes(`${path.sep}@typescript-eslint${path.sep}`)
      ) {
        delete require.cache[key]
      }
    }
    return await import('typescript-eslint')
  } finally {
    Module._resolveFilename = originalResolveFilename
  }
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
