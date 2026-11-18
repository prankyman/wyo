import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import config, { inlineScriptHashes, styleAttributeHashes } from '../svelte.config.js';

/**
 * The page's security policy (premise 6 of Plan 2): the browser may run the interface's own scripts and load nothing from any
 * other address. SvelteKit hashes only its own start-up script, so the two plain scripts of `src/app.html` are hashed by
 * `svelte.config.js` from the file itself, and this test fails when the hashes and the scripts' text part.
 */

/** The file SvelteKit writes its root component from; its announcer for screen readers carries the one inline style. */
const KIT_ROOT = readFileSync(path.resolve('node_modules/@sveltejs/kit/src/core/sync/write_root.js'), 'utf8');

const APP_HTML = readFileSync(path.resolve(import.meta.dirname, 'app.html'), 'utf8');
const csp = config.kit?.csp;
const directives = (csp?.directives ?? {}) as Record<string, unknown>;

/** The text of every inline script of a page, found by cutting the page at its `<script` tags (not by the function under test). */
function inlineScripts(html: string): string[] {
    return html
        .split('<script')
        .slice(1)
        .map((part) => part.slice(part.indexOf('>') + 1, part.indexOf('</script>')))
        .filter((text) => text.trim() !== '');
}

describe('the content security policy of the built interface', () => {
    it('holds every inline script of src/app.html by the hash of its current text, and the two scripts that are there', () => {
        const scripts = inlineScripts(APP_HTML);
        const hashes = scripts.map((text) => `sha256-${createHash('sha256').update(text).digest('base64')}`);

        expect(scripts).toHaveLength(2);
        expect(directives['script-src']).toEqual(['self', ...hashes]);
    });

    it("lets through exactly the style attribute of SvelteKit's own announcer and no other inline style", () => {
        const attribute = /id="svelte-announcer"[^>]*style="([^"]*)"/.exec(KIT_ROOT)?.[1];
        const hash = `sha256-${createHash('sha256')
            .update(attribute ?? '')
            .digest('base64')}`;

        expect(attribute).toContain('position: absolute');
        expect(directives['style-src-attr']).toEqual(['unsafe-hashes', hash]);
    });

    it('lets the page load and call nothing from another address and run no script it did not carry', () => {
        expect(csp?.mode).toBe('hash');
        expect(directives).toMatchObject({
            'default-src': ['self'],
            'style-src': ['self'],
            'img-src': ['self', 'data:'],
            'connect-src': ['self'],
            'object-src': ['none'],
            'base-uri': ['self'],
            'form-action': ['self']
        });
        expect(JSON.stringify(directives)).not.toContain('unsafe-inline');
        expect(JSON.stringify(directives)).not.toContain('unsafe-eval');
    });
});

describe('styleAttributeHashes', () => {
    it('hashes the value of every style attribute of a text', () => {
        const hash = (text: string) => `sha256-${createHash('sha256').update(text).digest('base64')}`;

        expect(styleAttributeHashes('<div style="a: 1">x</div><p class="c" style="b: 2"></p><i></i>')).toEqual([
            hash('a: 1'),
            hash('b: 2')
        ]);
    });
});

describe('inlineScriptHashes', () => {
    it('hashes the text of each script that has one, and skips a script that only points at a file', () => {
        const html = '<script>a()</script><script src="/x.js"></script><script type="module">b()</script>';
        const hash = (text: string) => `sha256-${createHash('sha256').update(text).digest('base64')}`;

        expect(inlineScriptHashes(html)).toEqual([hash('a()'), hash('b()')]);
    });
});
