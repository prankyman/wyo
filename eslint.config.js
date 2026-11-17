import eslint from '@eslint/js';
import eslintConfigPrettier from 'eslint-config-prettier';
import prettier from 'eslint-plugin-prettier';
import eslintPluginSvelte from 'eslint-plugin-svelte';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default [
    {
        ignores: [
            '.svelte-kit/',
            '.agents/',
            '.husky',
            '.tools/',
            'build/',
            'bin/',
            'coverage/',
            'node_modules/',
            // The editor's local-history backups: half-saved copies of files, never code. ESLint's flat config does not
            // read .gitignore, so they are listed.
            '.history/',
            'playwright-report/',
            'test-results/',
            // The interface as built, and the Go side: neither is checked by ESLint.
            'internal/',
            'cmd/',
            'tools/',
            // Saved from the outside, kept as published.
            'docs/marketplaces/',
            'api/',
            // Generated files, never edited by hand.
            'src/lib/api/wyo-api.d.ts',
            '**/gen/**',
            // The scripts are shell and plain Node programs outside the interface.
            'scripts/'
        ]
    },
    eslint.configs.recommended,
    ...tseslint.configs.recommended,
    ...eslintPluginSvelte.configs['flat/recommended'],
    {
        files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
        languageOptions: {
            parserOptions: {
                parser: tseslint.parser
            }
        }
    },
    {
        languageOptions: {
            globals: {
                ...globals.browser
            }
        },
        plugins: {
            prettier
        },
        rules: {
            'prettier/prettier': 'error',
            // A leading underscore means "this parameter exists because the signature requires it, and this function
            // does not read it". Without this the rule flags the deliberate marker.
            '@typescript-eslint/no-unused-vars': [
                'error',
                { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' }
            ],
            // The interface has no base path (svelte.config.js sets no `paths`), so wrapping every link in resolve()
            // would change no address.
            'svelte/no-navigation-without-resolve': 'off',
            // Every hit would be a throwaway object built inside a function to form one link or request; none of
            // them is screen state.
            'svelte/prefer-svelte-reactivity': 'off'
        }
    },
    {
        files: ['e2e/**/*.ts', '*.config.{js,ts}'],
        languageOptions: {
            globals: {
                ...globals.node
            }
        }
    },
    {
        // "One way in" for the network: the interface talks to nothing but the wyo server's own API, and every call is
        // made inside `src/lib/api/`, so the one place can be checked, stubbed and counted. A test file may spy on or
        // replace `fetch` to prove a call; the browser tests drive the real thing.
        files: ['src/**/*.svelte', 'src/**/*.ts', 'e2e/**/*.ts'],
        ignores: ['src/lib/api/**', 'src/**/*.test.ts', 'src/**/*.svelte.test.ts', 'src/test/**'],
        rules: {
            'no-restricted-globals': [
                'error',
                {
                    name: 'fetch',
                    message:
                        'Call the wyo server through src/lib/api/ (`api.ts`); `fetch` is used in that folder only, so every request the interface makes is in one place.'
                }
            ],
            'no-restricted-properties': [
                'error',
                {
                    object: 'window',
                    property: 'fetch',
                    message: 'Call the wyo server through src/lib/api/ (`api.ts`); `fetch` is used in that folder only.'
                },
                {
                    object: 'globalThis',
                    property: 'fetch',
                    message: 'Call the wyo server through src/lib/api/ (`api.ts`); `fetch` is used in that folder only.'
                }
            ]
        }
    },
    {
        // "One way in" for forms: the form library is imported by one file, the project's wrapper, so a form behaves
        // the same everywhere (single-page mode, the refusal laid onto the fields, a report of a call that never
        // arrived). A test may import the library to build a stand-in.
        files: ['src/**/*.svelte', 'src/**/*.ts'],
        ignores: ['src/lib/forms/super-form.ts', 'src/**/*.test.ts', 'src/**/*.svelte.test.ts', 'src/test/**'],
        rules: {
            'no-restricted-imports': [
                'error',
                {
                    patterns: [
                        {
                            group: ['sveltekit-superforms', 'sveltekit-superforms/*'],
                            allowTypeImports: true,
                            message:
                                "Import from '$lib/forms/super-form' instead; it is the only file that imports the form library. Type-only imports (SuperForm, SuperValidated, FormOptions) may still come from 'sveltekit-superforms'."
                        }
                    ]
                }
            ]
        }
    },
    {
        // A regrowth guard, not a gate: a file over 800 lines of code is a failure, to be split along its seams. Blank
        // lines and comments do not count. `pnpm lint` runs with --max-warnings 0, so the warning fails the build.
        // Test code is exempt: a test file is a flat catalogue of independent cases, and src/test/** is test code by
        // another name.
        files: ['src/**/*.svelte', 'src/**/*.ts'],
        ignores: ['src/**/*.test.ts', 'src/**/*.svelte.test.ts', 'src/test/**'],
        rules: {
            'max-lines': ['warn', { max: 800, skipBlankLines: true, skipComments: true }]
        }
    },
    eslintConfigPrettier
];
