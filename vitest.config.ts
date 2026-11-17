import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';
import { defineConfig } from 'vitest/config';

/**
 * The suite is split into two Vitest projects because many test files never touch a DOM, yet a single global
 * `environment: 'jsdom'` makes every one of them build a full jsdom (hundreds of milliseconds of CPU each) and load the whole
 * @testing-library import graph.
 *
 * Routing is by filename, which the sveltekit-testing skill mandates:
 *   *.svelte.test.ts -> jsdom (component tests)
 *   everything else  -> node  (pure logic, the text system, the call client, the policy tests that read source files)
 *
 * `isolate: false` is deliberately NOT used in either project: component tests leak module and DOM state across files, and a
 * file that mocks `$lib/i18n/i18n` silently wins for whichever file runs after it in the same worker.
 */

const alias = {
    $lib: path.resolve(import.meta.dirname, './src/lib'),
    // Shared test fixtures. Import through this alias so `vi.mock` factories can use a stable specifier regardless of how deep
    // the test file sits.
    $test: path.resolve(import.meta.dirname, './src/test'),
    '@': path.resolve(import.meta.dirname, './src'),
    '$app/environment': path.resolve(import.meta.dirname, './src/test/mocks/$app/environment.ts'),
    '$app/state': path.resolve(import.meta.dirname, './src/test/mocks/$app/state.ts'),
    '$app/navigation': path.resolve(import.meta.dirname, './src/test/mocks/$app/navigation.ts'),
    '$app/stores': path.resolve(import.meta.dirname, './src/test/mocks/$app/stores.ts'),
    // The form library imports these two itself, in every mode.
    '$app/forms': path.resolve(import.meta.dirname, './src/test/mocks/$app/forms.ts')
};

// The node project swaps in `browser = false`: tests that never touch a DOM must not run code that reads `document`.
const nodeAlias = {
    ...alias,
    '$app/environment': path.resolve(import.meta.dirname, './src/test/mocks/$app/environment.node.ts')
};

/**
 * Non-component tests that legitimately need a DOM, and so run in the jsdom project despite not being named
 * `*.svelte.test.ts`. Keep this list short and justified: it is the documented escape hatch, not a dumping ground.
 */
const DOM_DEPENDENT_NON_COMPONENT_TESTS = [
    'src/lib/components/ui/overlay-a11y.test.ts' // focus trap / inert / body-scroll helpers operate on real DOM nodes
];

// Both are root-level only: Vitest lists them in NonProjectOptions, so a copy inside a projects[] entry is a type error and is
// silently dropped at runtime.
const resolveSnapshotPath = (testPath: string, snapshotExtension: string) =>
    path.join(path.dirname(testPath), '__snapshots__', path.basename(testPath) + '.' + snapshotExtension);

const onConsoleLog = (log: string) => {
    // Svelte dev warning that can fire when a component's internal effect reads a $derived after test cleanup unmounts it.
    // Non-actionable noise.
    if (log.includes('derived_inert')) {
        return false;
    }
};

export default defineConfig({
    test: {
        onConsoleLog,
        resolveSnapshotPath,
        coverage: {
            provider: 'v8',
            reporter: ['text-summary', 'json-summary', 'json', 'lcov'],
            /**
             * `include` is load-bearing, not decoration. The v8 provider only instruments files that were actually LOADED during
             * the run, so without this list a file with no test is not "below the floor": it is absent from the report entirely,
             * counted in neither the aggregate nor `scripts/coverage-check.cjs`'s per-file gate.
             *
             * Every entry in `exclude` below carries its own written reason.
             */
            include: ['src/**/*.{ts,svelte}'],
            exclude: [
                // Test scaffolding and fixtures - not production code.
                'src/test/**',
                '**/*.test.{js,ts}',
                '**/*.spec.{js,ts}',
                '**/node_modules/**',
                '**/.git/**',
                '**/*.css',
                // Playwright specs run under a different runner entirely.
                'e2e/**',

                // Type-only modules: zero runtime statements, so they can only ever report 0/0.
                '**/types.ts',
                '**/*-types.ts',
                '**/*.d.ts'
            ]
        },
        projects: [
            {
                plugins: [svelte()],
                resolve: {
                    conditions: process.env.VITEST ? ['browser'] : undefined,
                    alias: nodeAlias
                },
                test: {
                    name: 'node',
                    environment: 'node',
                    pool: 'threads',
                    setupFiles: './src/test/setup.node.ts',
                    globals: true,
                    include: ['src/**/*.{test,spec}.{js,ts}'],
                    exclude: ['**/node_modules/**', '**/*.svelte.test.ts', ...DOM_DEPENDENT_NON_COMPONENT_TESTS]
                }
            },
            {
                plugins: [svelte()],
                resolve: {
                    conditions: process.env.VITEST ? ['browser'] : undefined,
                    alias
                },
                test: {
                    name: 'jsdom',
                    environment: 'jsdom',
                    // pool/isolate left at Vitest defaults (forks, isolate: true) on purpose: see the note at the top of this file.
                    setupFiles: './src/test/setup.ts',
                    globals: true,
                    include: ['src/**/*.svelte.test.ts', ...DOM_DEPENDENT_NON_COMPONENT_TESTS],
                    exclude: ['**/node_modules/**']
                }
            }
        ]
    }
});
