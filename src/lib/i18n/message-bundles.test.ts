import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The central guard on the completeness of the texts. A component test proves only that a component's language plumbing works,
 * never that an individual text is translated; these few tests cover every key of every file in `messages/`, and every key
 * written anywhere in `src/`.
 *
 * Today `messages/ru.json` is the only file; the checks that compare files with each other cost nothing then and start
 * working the day a second file is added. No test asserts the same text in two languages.
 */

const ROOT = path.resolve(__dirname, '../../..');
const MESSAGES_DIR = path.join(ROOT, 'messages');
const SRC_DIR = path.join(ROOT, 'src');
const PLURAL_CATEGORIES = ['one', 'few', 'many', 'other'];

interface IBundle {
    locale: string;
    texts: Record<string, unknown>;
    keys: Set<string>;
}

const bundles: IBundle[] = fs
    .readdirSync(MESSAGES_DIR)
    .filter((name) => name.endsWith('.json'))
    .map((name) => {
        const texts = JSON.parse(fs.readFileSync(path.join(MESSAGES_DIR, name), 'utf8')) as Record<string, unknown>;
        return { locale: path.basename(name, '.json'), texts, keys: new Set(Object.keys(texts)) };
    });

const russian = bundles.find((bundle) => bundle.locale === 'ru');

/** Every `.ts`/`.svelte` file under src/ that is not a test or test support. */
function sourceFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
            if (full !== path.join(SRC_DIR, 'test')) sourceFiles(full, acc);
        } else if (/\.(ts|svelte)$/.test(entry.name) && !/\.(test|spec)\.ts$/.test(entry.name)) {
            acc.push(full);
        }
    }
    return acc;
}

/** The source without comments, so a key named in prose is not taken for a use. */
function withoutComments(source: string): string {
    return source
        .replace(/<!--[\s\S]*?-->/g, '')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/(^|[^:'"`])\/\/.*$/gm, '$1');
}

/**
 * Literal keys passed to `t()` and `tError()`, and the base keys passed to `tPlural()`. Template literals and computed keys are
 * skipped on purpose: a key built from data cannot be checked statically, which is exactly why `hasMessage()` exists.
 */
function literalKeysUsedIn(source: string): { keys: string[]; pluralBases: string[] } {
    const code = withoutComments(source);
    const keys = [...code.matchAll(/\b(?:t|tError)\(\s*'([a-zA-Z][a-zA-Z0-9_.-]*)'/g)].map((match) => match[1]);
    const pluralBases = [...code.matchAll(/\btPlural\(\s*'([a-zA-Z][a-zA-Z0-9_.-]*)'/g)].map((match) => match[1]);
    return { keys, pluralBases };
}

/** The `{name}` placeholders of a text. */
const placeholdersOf = (text: unknown): string[] =>
    typeof text === 'string' ? [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort() : [];

describe('message bundles', () => {
    it('has the default language, Russian', () => {
        expect(russian, 'messages/ru.json is the default language and must exist').toBeDefined();
    });

    it('gives every language exactly the same key set', () => {
        const [reference, ...others] = bundles;

        for (const other of others) {
            const missing = [...reference.keys].filter((key) => !other.keys.has(key)).sort();
            const extra = [...other.keys].filter((key) => !reference.keys.has(key)).sort();

            expect(
                missing,
                `messages/${other.locale}.json is missing ${missing.length} key(s) present in messages/${reference.locale}.json, e.g. ${missing.slice(0, 5).join(', ')}`
            ).toEqual([]);
            expect(
                extra,
                `messages/${other.locale}.json has ${extra.length} key(s) absent from messages/${reference.locale}.json, e.g. ${extra.slice(0, 5).join(', ')}`
            ).toEqual([]);
        }
    });

    it('fills the same {placeholders} in every language', () => {
        const [reference, ...others] = bundles;
        const different = others.flatMap((other) =>
            [...reference.keys]
                .filter(
                    (key) =>
                        other.keys.has(key) &&
                        placeholdersOf(reference.texts[key]).join() !== placeholdersOf(other.texts[key]).join()
                )
                .map((key) => `${key} (${other.locale})`)
        );

        expect(different, 'a text names other {placeholders} than the same text in the first language').toEqual([]);
    });

    it('has a non-empty string for every key in every language', () => {
        for (const { locale, texts } of bundles) {
            const blank = Object.entries(texts)
                .filter(([, value]) => typeof value !== 'string' || value.trim() === '')
                .map(([key]) => key);

            expect(
                blank,
                `messages/${locale}.json has blank/non-string values: ${blank.slice(0, 5).join(', ')}`
            ).toEqual([]);
        }
    });

    /**
     * A word whose letters come from two alphabets is a typo that proofreading cannot see. A Latin `A` (U+0041) draws
     * identically to a Cyrillic `А` (U+0410), so a label typed with one Latin letter among Cyrillic ones looks right on screen
     * while being a different string from the one any other code writes.
     */
    it('has no word mixing Cyrillic and Latin letters in any bundle', () => {
        const mixed: string[] = [];

        for (const { locale, texts } of bundles) {
            for (const [key, value] of Object.entries(texts)) {
                for (const [word] of String(value).matchAll(/\p{L}+/gu)) {
                    if (/\p{Script=Cyrillic}/u.test(word) && /\p{Script=Latin}/u.test(word)) {
                        mixed.push(`${locale}.json - ${key}: "${word}"`);
                    }
                }
            }
        }

        expect(
            mixed,
            `${mixed.length} word(s) mix Cyrillic and Latin letters; retype the whole word in one alphabet: ${mixed.slice(0, 5).join(', ')}`
        ).toEqual([]);
    });

    it('gives every plural base all four plural categories in every language', () => {
        const incomplete: string[] = [];

        for (const { locale, keys } of bundles) {
            const bases = new Set(
                [...keys]
                    .filter((key) => PLURAL_CATEGORIES.some((category) => key.endsWith(`.${category}`)))
                    .map((key) => key.slice(0, key.lastIndexOf('.')))
            );
            for (const base of bases) {
                for (const category of PLURAL_CATEGORIES) {
                    if (!keys.has(`${base}.${category}`)) incomplete.push(`${base}.${category} (${locale})`);
                }
            }
        }

        expect(incomplete).toEqual([]);
    });

    it('resolves every literal t() key used in src/ in every language', () => {
        const files = sourceFiles(SRC_DIR);
        const unresolved: string[] = [];

        for (const file of files) {
            const { keys, pluralBases } = literalKeysUsedIn(fs.readFileSync(file, 'utf8'));
            for (const bundle of bundles) {
                for (const key of keys) {
                    if (!bundle.keys.has(key))
                        unresolved.push(`${key} (${bundle.locale}) - ${path.relative(ROOT, file)}`);
                }
                for (const base of pluralBases) {
                    if (!bundle.keys.has(`${base}.other`))
                        unresolved.push(`${base}.other (${bundle.locale}) - ${path.relative(ROOT, file)}`);
                }
            }
        }

        expect(files.length).toBeGreaterThan(5);
        expect(
            [...new Set(unresolved)].sort(),
            `t() is called with ${unresolved.length} key(s) that no bundle defines; they would render as the raw key`
        ).toEqual([]);
    });

    /**
     * The same question asked the other way round, because the scan above can only see a key written INSIDE a `t()` call - and
     * most screens do not write them there. A column definition carries a `labelKey`, a form is handed a `submitKey`, a card
     * chooses between two labels and passes the winner; each is rendered with `t(someVariable)`, which the scan above skips.
     *
     * So this checks something blunter: ANY quoted word in the source that is shaped like one of our keys - it starts with a
     * top-level name of `messages/ru.json`, such as `errors.` or `start.` - must exist in every bundle. It cannot be evaded by
     * moving a key out of the call, and a string that only looks like a key goes on the allow-list below with its reason.
     */
    it('resolves every key-shaped string written anywhere in src/, however it reaches t()', () => {
        const NOT_A_MESSAGE_KEY = new Map<string, string>([
            [
                'accounts.refusal',
                'the family of the texts of the accounts’ refusals (`src/lib/marketplaces/refusals.ts`); `refusalKey` adds the code'
            ]
        ]);
        const names = new Set([...(russian?.keys ?? [])].map((key) => key.split('.')[0]));
        const KEY_SHAPED = new RegExp(`['"\`]((?:${[...names].join('|')})\\.[A-Za-z0-9_.]+)['"\`]`, 'g');
        const unresolved: string[] = [];

        for (const file of sourceFiles(SRC_DIR)) {
            for (const match of withoutComments(fs.readFileSync(file, 'utf8')).matchAll(KEY_SHAPED)) {
                const key = match[1];
                if (NOT_A_MESSAGE_KEY.has(key)) continue;
                for (const bundle of bundles) {
                    if (!bundle.keys.has(key) && !bundle.keys.has(`${key}.other`))
                        unresolved.push(`${key} (${bundle.locale}) - ${path.relative(ROOT, file)}`);
                }
            }
        }

        expect(
            [...new Set(unresolved)].sort(),
            `${unresolved.length} key-shaped string(s) name no text; they would render as the raw key`
        ).toEqual([]);
    });

    it('finds the keys a source uses, and not the ones its comments mention', () => {
        const found = literalKeysUsedIn(
            "const a = t('a.one'); const b = tError('b.two', lang);\nconst c = tPlural('c.days', 3);\n// t('d.comment')\nconst e = hasMessage('e.no');"
        );

        expect(found.keys).toEqual(['a.one', 'b.two']);
        expect(found.pluralBases).toEqual(['c.days']);
    });
});
