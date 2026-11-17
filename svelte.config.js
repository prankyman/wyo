import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
    compilerOptions: {
        // Force runes mode for the project, except for libraries. Can be removed in svelte 6.
        runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
    },
    kit: {
        // Static files with a fallback page and no server side: the wyo server (Go) carries the built files inside
        // itself and answers every address that is not `/api/` with `index.html`. No page is prerendered.
        adapter: adapter({
            pages: 'internal/webui/dist',
            assets: 'internal/webui/dist',
            fallback: 'index.html',
            strict: false
        }),
        alias: {
            $lib: './src/lib',
            '@': './src',
            // Shared test fixtures (src/test/support). Declared here so `svelte-kit sync` emits the tsconfig path and
            // `pnpm check` resolves it; no production code imports it.
            $test: './src/test'
        }
    }
};

export default config;
