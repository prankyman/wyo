import type { Reroute } from '@sveltejs/kit';
import { DEFAULT_LOCALE, getActiveLocales } from '$lib/i18n/messages';
import { splitLocale } from '$lib/i18n/locale-path';
import { z } from 'zod';

// The form checks (zod) would otherwise try once whether they may build functions at run time (`new Function`); the page's
// security policy refuses that and the browser reports a violation on every page, even though zod catches it. `jitless`
// makes zod check without building functions (https://zod.dev, the page «AOT compilation», section «Content Security Policy»). This runs before any page's code.
z.config({ jitless: true });

/**
 * A page of a second language is reached by an address `/xx/...` and is the same page as `/...`: this maps the address to
 * the route before SvelteKit matches it (https://svelte.dev/docs/kit/hooks#Universal-hooks-reroute). The address in the
 * browser stays as it is, and the language of the page is read from it (`getLocale`, `src/lib/i18n/i18n.ts`). A language
 * exists when `messages/xx.json` does, so with the Russian file alone no address is changed.
 */
export const reroute: Reroute = ({ url }) => {
    const { locale, path } = splitLocale(url.pathname, getActiveLocales(), DEFAULT_LOCALE);
    if (locale !== DEFAULT_LOCALE) return path;
};
