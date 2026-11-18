import adapter from '@sveltejs/adapter-static';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';

/**
 * The CSP hashes (`sha256-…`) of the inline scripts of a page: the SHA-256 of each script's exact text, in base64. A script that
 * only points at a file (`<script src>`) has no text and no hash. SvelteKit hashes its own start-up script itself but not the
 * scripts of `src/app.html`, so they are hashed here, from the file, every time the configuration is read: a changed script gets
 * its new hash with no step to remember (`src/csp.test.ts` fails when the two part).
 * @param {string} html
 * @returns {`sha256-${string}`[]}
 */
export function inlineScriptHashes(html) {
    return [...html.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/g)]
        .map((match) => match[1])
        .filter((text) => text.trim() !== '')
        .map(
            (text) => /** @type {`sha256-${string}`} */ (`sha256-${createHash('sha256').update(text).digest('base64')}`)
        );
}

/**
 * The CSP hashes of the values of every `style="…"` attribute of a text. SvelteKit's root component (written from
 * `@sveltejs/kit/src/core/sync/write_root.js`) draws its announcer for screen readers with one literal style attribute, which
 * `style-src 'self'` refuses; that one value is let through by its hash (`'unsafe-hashes'` lets a hash cover an attribute),
 * read from SvelteKit's own file, so an upgrade that changes it changes the hash too. No other inline style is allowed.
 * @param {string} text
 * @returns {`sha256-${string}`[]}
 */
export function styleAttributeHashes(text) {
    return [...text.matchAll(/\sstyle="([^"]*)"/g)].map(
        (match) =>
            /** @type {`sha256-${string}`} */ (`sha256-${createHash('sha256').update(match[1]).digest('base64')}`)
    );
}

const KIT_ROOT = readFileSync(
    new URL('./node_modules/@sveltejs/kit/src/core/sync/write_root.js', import.meta.url),
    'utf8'
);

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
        // The page's security policy (https://svelte.dev/docs/kit/configuration#csp): the interface runs its own scripts and
        // loads nothing from any other address. `mode: 'hash'` writes the policy into the page as a `<meta>` tag with the
        // hashes of SvelteKit's start-up script; the two plain scripts of `src/app.html` are added by `inlineScriptHashes`.
        // `frame-ancestors` is left out on purpose: a `<meta>` policy ignores it.
        csp: {
            mode: 'hash',
            directives: {
                'default-src': ['self'],
                'script-src': [
                    'self',
                    ...inlineScriptHashes(readFileSync(new URL('./src/app.html', import.meta.url), 'utf8'))
                ],
                'style-src': ['self'],
                'style-src-attr': ['unsafe-hashes', ...styleAttributeHashes(KIT_ROOT)],
                'img-src': ['self', 'data:'],
                'connect-src': ['self'],
                'object-src': ['none'],
                'base-uri': ['self'],
                'form-action': ['self']
            }
        },
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
