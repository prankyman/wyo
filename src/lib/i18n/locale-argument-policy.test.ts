import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The language of a page is the one in its address, and nothing else decides it. Enforced against the source rather than
 * against a reviewer's memory.
 *
 * Three families of bug this stops coming back:
 *
 * 1. **A language written by hand.** `lang === 'ru'`, `locale: 'en'`, a table of languages: the day a second file is added to
 *    `messages/`, every such place is wrong. The text system (`src/lib/i18n/`) is the one place that knows `DEFAULT_LOCALE`;
 *    everywhere else the language is a value that arrived (`lang`, `locale`).
 *
 * 2. **A date formatted in a locale the page is not in.** `'en-GB'`, `lang === 'ru' ? 'ru-RU' : 'en-GB'`, or nothing at all
 *    (which falls back to the language of the computer). The locale IS the language code of the page - `lang` or `locale`,
 *    whichever the surrounding code calls it - never a regional tag, never a ternary, never omitted.
 *
 * 3. **A number formatted in a locale nobody passed.** `Intl.NumberFormat()` with no locale prints with the computer's
 *    grouping, so the same amount reads differently on two machines.
 */
const SRC = 'src';
const HOME = join('src', 'lib', 'i18n');

/** Languages that a seller's interface could be offered in; none of them may be written as a bare quoted code outside the text system. */
const LANGUAGE_CODES = [
    'ru',
    'en',
    'zh',
    'de',
    'fr',
    'es',
    'it',
    'pt',
    'uk',
    'tr',
    'ar',
    'ja',
    'ko',
    'pl',
    'cs',
    'kk',
    'be'
];
const LANGUAGE_LITERAL = new RegExp(`(?<![\\w-])['"\`](?:${LANGUAGE_CODES.join('|')})['"\`](?![\\w-])`);
const REGIONAL_TAG = /['"`][a-z]{2}-[A-Z]{2}['"`]/;
const DATE_FN_WITH_LITERAL = /\bformatDate(?:Time)?\(\s*[^,()]*,\s*['"]/;
const TO_LOCALE_WITH_LITERAL = /\.toLocale(?:Date|Time)?String\(\s*['"]/;
const TO_LOCALE_WITH_NOTHING = /\.toLocale(?:Date|Time)?String\(\s*\)/;
const INTL_WITH_LITERAL = /\bIntl\.(?:DateTimeFormat|NumberFormat|PluralRules|ListFormat|RelativeTimeFormat)\(\s*['"]/;
const INTL_WITH_NOTHING = /\bIntl\.(?:DateTimeFormat|NumberFormat)\(\s*\)/;
/** Date calls, to count their arguments. */
const DATE_FN = /\b(formatDate|formatDateTime)\(/g;

/**
 * Returns the text between a call's parentheses, following it across line breaks and skipping nested calls, so a formatter
 * split over five lines is read as one argument list. `from` is the index of the opening paren.
 */
function argumentList(source: string, from: number): string {
    let depth = 0;
    for (let i = from; i < source.length; i += 1) {
        const ch = source[i];
        if (ch === '(') depth += 1;
        else if (ch === ')') {
            depth -= 1;
            if (depth === 0) return source.slice(from + 1, i);
        }
    }
    return source.slice(from + 1);
}

/** Splits an argument list on top-level commas - nested calls, objects and arrays stay whole. */
function topLevelArguments(list: string): string[] {
    const out: string[] = [];
    let depth = 0;
    let start = 0;
    for (let i = 0; i < list.length; i += 1) {
        const ch = list[i];
        if (ch === '(' || ch === '[' || ch === '{') depth += 1;
        else if (ch === ')' || ch === ']' || ch === '}') depth -= 1;
        else if (ch === ',' && depth === 0) {
            out.push(list.slice(start, i));
            start = i + 1;
        }
    }
    out.push(list.slice(start));
    return out.map((argument) => argument.trim()).filter((argument) => argument.length > 0);
}

/** Line number of a character offset, 1-indexed. */
function lineOf(source: string, index: number): number {
    return source.slice(0, index).split('\n').length;
}

/** The source with its comments blanked, so a rule written out in prose is not taken for a call. */
function withoutComments(source: string): string {
    return source
        .replace(/<!--[\s\S]*?-->/g, (comment) => comment.replace(/[^\n]/g, ' '))
        .replace(/\/\*[\s\S]*?\*\//g, (comment) => comment.replace(/[^\n]/g, ' '))
        .replace(/(^|[^:'"`])\/\/.*$/gm, '$1');
}

function sourceFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) {
            if (entry.name !== 'test' && full !== HOME) sourceFiles(full, acc);
        } else if (/\.(ts|svelte)$/.test(entry.name) && !/\.(test|spec)\.ts$/.test(entry.name)) {
            acc.push(full);
        }
    }
    return acc;
}

/** What is wrong with the language handling of one source, one finding per line. */
export function violations(file: string, source: string): string[] {
    const out: string[] = [];
    const code = withoutComments(source);

    code.split('\n').forEach((line, i) => {
        const at = `${file}:${i + 1}`;
        if (LANGUAGE_LITERAL.test(line)) {
            out.push(
                `${at} writes a language code by hand - the language comes from the page's address (getLocale, lang)`
            );
        }
        if (REGIONAL_TAG.test(line)) {
            out.push(`${at} names a region tag - the Intl locale is the language code of the page`);
        }
        if (DATE_FN_WITH_LITERAL.test(line) || INTL_WITH_LITERAL.test(line) || TO_LOCALE_WITH_LITERAL.test(line)) {
            out.push(`${at} formats in a hard-coded locale - pass lang/locale`);
        }
        if (TO_LOCALE_WITH_NOTHING.test(line) || INTL_WITH_NOTHING.test(line)) {
            out.push(`${at} formats in the runtime's own locale - pass lang/locale`);
        }
    });

    for (const match of code.matchAll(DATE_FN)) {
        const open = match.index! + match[0].length - 1;
        if (topLevelArguments(argumentList(code, open)).length < 2) {
            out.push(`${file}:${lineOf(code, match.index!)} formats a date with no locale - pass lang/locale`);
        }
    }

    return out;
}

describe('the language of a page is the one in its address', () => {
    it('finds each kind of fault, and lets a value that arrived through', () => {
        const bad = [
            "const dutch = lang === 'ru';",
            "const f = { locale: 'en' };",
            "const tag = 'en-GB';",
            "formatDate(value, 'ru')",
            "new Date(x).toLocaleDateString('ru', {})",
            "new Intl.DateTimeFormat('ru')",
            'new Date(x).toLocaleString()',
            'new Intl.NumberFormat()',
            'formatDateTime(\n    value\n)'
        ];
        for (const source of bad) expect(violations('x', source), source).not.toEqual([]);

        const good = [
            'formatDate(value, lang)',
            'formatDate(\n    value,\n    lang\n)',
            'new Intl.NumberFormat(locale, { style: "decimal" })',
            "// lang === 'ru' is exactly what is forbidden",
            "const word = 'running';",
            "const name = 'ru-ning'"
        ];
        for (const source of good) expect(violations('x', source), source).toEqual([]);
    });

    it('writes no language code, no region tag and no hard-coded or missing locale anywhere outside the text system', () => {
        const files = sourceFiles(SRC);
        expect(files.length).toBeGreaterThan(5);

        const found = files.flatMap((file) => violations(file, readFileSync(file, 'utf8')));

        expect(found, found.join('\n')).toEqual([]);
    });
});
