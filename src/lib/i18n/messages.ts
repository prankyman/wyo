import type { TLocale } from './types';

/**
 * The one language written in code (Principle: language). Every other language exists only because a file
 * `messages/<code>.json` exists; adding one needs no change in any `.ts` or `.svelte` file.
 */
export const DEFAULT_LOCALE: TLocale = 'ru';

type Messages = Record<string, string>;

// Message files are found by pattern, not imported one by one: a new `messages/xx.json` is a new language with no code change.
const modules = import.meta.glob<{ default: Messages }>('../../../messages/*.json', { eager: true });

/** The texts of every language, by language code. The table is exported so a test can add a language of its own. */
export const messages: Record<TLocale, Messages> = {};

for (const [path, module] of Object.entries(modules)) {
    const code = path.match(/([^/]+)\.json$/)?.[1];
    if (code) messages[code] = module.default;
}

/** The languages that exist right now: the default first, then the others in alphabetical order. */
export function getActiveLocales(): readonly TLocale[] {
    const others = Object.keys(messages)
        .filter((code) => code !== DEFAULT_LOCALE)
        .sort();
    return messages[DEFAULT_LOCALE] ? [DEFAULT_LOCALE, ...others] : others;
}

/** Whether `code` names a language that exists. */
export function isLocale(code: string): code is TLocale {
    return code in messages;
}
