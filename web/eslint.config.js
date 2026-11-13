import js from '@eslint/js';
import boundaries from 'eslint-plugin-boundaries';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['node_modules', 'coverage'] },
  js.configs.recommended,
  tseslint.configs.recommended,
  reactHooks.configs.flat.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: { globals: globals.browser },
    plugins: { boundaries },
    settings: {
      'import/resolver': { typescript: { alwaysTryTypes: true } },
      'boundaries/legacy-templates': false,
      'boundaries/elements': [
        { type: 'app', pattern: 'src/app', partialMatch: false },
        {
          type: 'feature',
          pattern: 'src/features/*',
          capture: ['feature'],
          partialMatch: false,
        },
        { type: 'shared', pattern: 'src/shared', partialMatch: false },
      ],
    },
    rules: {
      // Architecture boundaries (AGENTS.md, "Padrões do frontend"):
      // app -> features -> shared, never the other way around, and one
      // feature reaches another only through its index.ts.
      'boundaries/dependencies': [
        'error',
        {
          default: 'disallow',
          policies: [
            {
              from: { element: { type: ['app', 'feature'] } },
              allow: { to: { element: { type: 'shared' } } },
            },
            {
              from: { element: { type: ['app', 'feature'] } },
              allow: {
                to: { element: { type: 'feature', fileInternalPath: 'index.ts' } },
              },
            },
          ],
        },
      ],
    },
  },
  {
    // Domain logic is plain TypeScript, testable without a browser.
    files: ['src/features/*/domain/**'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: ['react', 'react-dom'],
          patterns: ['react/*', 'react-dom/*'],
        },
      ],
    },
  },
);
