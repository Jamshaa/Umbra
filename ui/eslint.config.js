// Minimal lint gate: catch undefined identifiers at build time.
// `vite build` does not do scope analysis, so a dropped import or
// state variable ships a blank page with a green build (happened
// twice). This one rule exists to catch exactly that.
import globals from 'globals'

export default [
  {
    ignores: ['dist/**'],
  },
  {
    files: ['src/**/*.{js,jsx}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      parserOptions: { ecmaFeatures: { jsx: true } },
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      'no-undef': 'error',
    },
  },
]
