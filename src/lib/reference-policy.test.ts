import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Rule R2: the fulfilment-centre API is neutral. Its specification — the guide and the description file — is written in this
 * repository's own words and names no marketplace: a marketplace is only a code inside the data, which the account call lists.
 * A name found here would tie the neutral format to one marketplace.
 */

const SPECIFICATION_FILES = ['docs/fulfilment-centre-api.md', 'api/fulfilment-centre.yaml'];

/** The marketplaces' names, in Latin and Cyrillic letters, as whole words. */
const MARKETPLACE_NAMES =
    /(?<![\p{L}\p{N}])(wildberries|вайлдберриз|ozon|озон|yandex|яндекс|megamarket|мегамаркет|sbermegamarket|avito|авито|lemana|лемана|леруа|aliexpress|kazanexpress|\bwb\b)(?![\p{L}\p{N}])/giu;

function marketplaceNamesIn(text: string): string[] {
    return text
        .split('\n')
        .flatMap((line, index) =>
            [...line.matchAll(MARKETPLACE_NAMES)].map((match) => `line ${index + 1}: ${match[0]}`)
        );
}

describe('the neutral fulfilment-centre API names no marketplace (rule R2)', () => {
    it.each(SPECIFICATION_FILES)('%s names no marketplace', (file) => {
        const text = readFileSync(path.resolve(file), 'utf8');

        expect(marketplaceNamesIn(text)).toEqual([]);
    });

    it('catches a planted marketplace name in either alphabet, and leaves the word «marketplace» alone', () => {
        const planted = [
            'The marketplace code is a string.',
            'Orders of Wildberries arrive here.',
            'Заказы с Озон приходят сюда.',
            'A WB sticker.',
            'marketplaces and marketplace codes'
        ].join('\n');

        expect(marketplaceNamesIn(planted)).toEqual(['line 2: Wildberries', 'line 3: Озон', 'line 4: WB']);
    });
});
