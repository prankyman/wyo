import { readFileSync, readdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';
import { marketplaceNamesIn } from '$test/support/marketplace-names';

/**
 * Rule R6 in the wyo interface: the screens ask the adapter, never the marketplace's name. A marketplace is named only in the
 * texts of the family `marketplace.<code>.*` of `messages/`, which the interface finds by the adapter's code; a file under `src/`
 * that names a marketplace anywhere else — a label, a condition on the name, a comment, a test — fails here. A message key of that
 * family written in a file (`'marketplace.<code>.name'`) is a key, not a name, and passes.
 *
 * The exceptions, each with its reason, are the complete record of what does not follow the rule.
 */

const ROOT = join(import.meta.dirname, '../..');

const EXCEPTIONS: Readonly<Record<string, string>> = {
    'src/lib/marketplace-name-policy.test.ts': 'this test plants marketplace names to prove that it finds them',
    'src/lib/reference-policy.test.ts': 'the test of rule R2 plants marketplace names to prove that it finds them',
    'src/test/support/marketplace-names.ts': 'the one list of the names the two policy tests look for',
    'src/test/support/marketplaces.ts':
        'the one test fixture that holds an adapter’s code and name as the wyo server sends them; every test takes them from it',
    'src/lib/api/wyo-api.d.ts':
        'generated from api/wyo.yaml, whose descriptions give an adapter’s code as an example; never edited by hand'
};

/** A name that is part of a message key of the family `marketplace.<code>.…` is that key, not a name. */
const MESSAGE_KEY_PART = /marketplace\.[\p{L}\p{N}_-]+\./gu;

/** The names a source holds outside the keys of the family `marketplace.<code>.*`. */
export function namesOutsideTheirTexts(source: string): string[] {
    return marketplaceNamesIn(source.replace(MESSAGE_KEY_PART, 'marketplace.<code>.'));
}

function sourceFiles(dir: string, acc: string[] = []): string[] {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = join(dir, entry.name);
        if (entry.isDirectory()) sourceFiles(full, acc);
        else if (/\.(ts|svelte|html|css|js|json)$/.test(entry.name))
            acc.push(relative(ROOT, full).split('\\').join('/'));
    }
    return acc;
}

describe('a marketplace is named only in the texts of its family (rule R6)', () => {
    it('finds a name in a label, a condition and a comment, and passes a key of the family', () => {
        const planted = [
            "const label = t('marketplace.wildberries.name', lang);",
            "if (account.marketplace === 'wildberries') return;",
            '<span>Заказы Wildberries</span>',
            '// the Ozon adapter comes later',
            "const help = 'marketplace.ozon.key.token.help';"
        ].join('\n');

        expect(namesOutsideTheirTexts(planted)).toEqual(['line 2: wildberries', 'line 3: Wildberries', 'line 4: Ozon']);
    });

    it('finds no marketplace named in any file under src/ but the listed exceptions', () => {
        const files = sourceFiles(join(ROOT, 'src')).filter((file) => !(file in EXCEPTIONS));
        const named = files.flatMap((file) =>
            namesOutsideTheirTexts(readFileSync(join(ROOT, file), 'utf8')).map((found) => `${file} ${found}`)
        );

        expect(files.length).toBeGreaterThan(50);
        expect(named).toEqual([]);
    });

    it('keeps every exception true: a reason, and a file that still exists and still names a marketplace', () => {
        for (const [file, reason] of Object.entries(EXCEPTIONS)) {
            expect(reason.length, file).toBeGreaterThan(20);
            expect(marketplaceNamesIn(readFileSync(join(ROOT, file), 'utf8')).length, file).toBeGreaterThan(0);
        }
    });
});
