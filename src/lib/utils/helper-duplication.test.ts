import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Standing guarantee that a helper with one home is not re-declared locally: everything is a component, and a piece of logic
 * used twice is one function, called from every place that needs it. A source-parsing rule that survives a rebase. Add a name
 * here the day a second copy of it appears, or the day a helper is written that a second place is likely to need.
 */
const HOMES: Record<string, string> = {
    // The text system: dates, decimals, counted phrases, the language of an address, a refusal in words.
    formatDate: 'src/lib/i18n/format-date.ts',
    formatDateTime: 'src/lib/i18n/format-date.ts',
    formatDateParts: 'src/lib/i18n/format-date.ts',
    formatIsoDay: 'src/lib/i18n/format-date.ts',
    decimalComma: 'src/lib/i18n/format-number.ts',
    decimalText: 'src/lib/i18n/format-number.ts',
    tError: 'src/lib/i18n/form-error.ts',
    tPlural: 'src/lib/i18n/plural.ts',
    splitLocale: 'src/lib/i18n/locale-path.ts',
    withLocale: 'src/lib/i18n/locale-path.ts',
    stripLocale: 'src/lib/i18n/i18n.ts',
    localizePath: 'src/lib/i18n/i18n.ts',
    // The one place the interface calls the wyo server, and the words of a refusal.
    apiRequest: 'src/lib/api/api.ts',
    refusalParams: 'src/lib/api/api.ts',
    refusalKey: 'src/lib/api/refusal-key.ts',
    fieldRefusalKey: 'src/lib/api/refusal-key.ts',
    // Forms: the refusal laid onto the fields, the answer, the section's fingerprint.
    applyRefusal: 'src/lib/forms/super-form.ts',
    spaForm: 'src/lib/forms/super-form.ts',
    answerOutcome: 'src/lib/forms/answer.ts',
    answerValue: 'src/lib/forms/answer.ts',
    answerLines: 'src/lib/forms/answer.ts',
    sectionFingerprint: 'src/lib/forms/section-form.ts',
    // Small utilities.
    copyToClipboard: 'src/lib/utils/clipboard.ts',
    cn: 'src/lib/utils/cn.ts'
};

const NAMES = Object.keys(HOMES).join('|');
const DECLARATION = new RegExp(
    `^\\s*(?:export\\s+)?(?:async\\s+)?(?:function\\s+(${NAMES})\\s*[<(]|(?:const|let)\\s+(${NAMES})\\s*[:=])`,
    'gm'
);

function sourceFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) {
            if (entry.name !== 'test') sourceFiles(full, acc);
        } else if (/\.(ts|svelte)$/.test(entry.name) && !/\.(test|spec)\.ts$/.test(entry.name)) {
            acc.push(full);
        }
    }
    return acc;
}

describe('helper duplication', () => {
    it('keeps every home in the table real', () => {
        const missing = Object.entries(HOMES)
            .filter(
                ([name, home]) => !new RegExp(`(?:function|const|let)\\s+${name}\\b`).test(readFileSync(home, 'utf8'))
            )
            .map(([name, home]) => `${name} is not declared in ${home}`);

        expect(missing).toEqual([]);
    });

    it('declares each consolidated helper only in its home', () => {
        const strays: string[] = [];
        for (const file of sourceFiles('src')) {
            const source = readFileSync(file, 'utf8');
            for (const match of source.matchAll(DECLARATION)) {
                const name = match[1] ?? match[2];
                if (HOMES[name] !== file) strays.push(`${file} declares ${name} (home: ${HOMES[name]})`);
            }
        }
        expect(strays).toEqual([]);
    });

    it('catches a second copy', () => {
        const copy = 'export function formatDate(value: string) {\n    return value;\n}\n';
        expect([...copy.matchAll(DECLARATION)].map((match) => match[1])).toEqual(['formatDate']);
    });
});
