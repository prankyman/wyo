import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());

import { hasMessage } from '$lib/i18n/i18n';
import { tPlural } from './plural';

/**
 * No bundle carries a counted phrase yet, so these tests prove the helper's own logic — which
 * CLDR category a count selects in each language, and the fallback when that category has no
 * key — rather than a bundle's wording. `hasMessage` is the shared mock's `vi.fn`, driven here
 * to say which category keys "exist"; `t()` hands an unknown key back unchanged, which is what
 * makes the selected key observable.
 */
const CATEGORY_KEYS = /\.(one|few|many|other)$/;

describe('tPlural', () => {
    beforeEach(() => {
        vi.mocked(hasMessage).mockReset();
    });

    // The whole point of the helper. Russian picks by the last digit and by 11–14, not by
    // "is it one": 21 reads like 1, 22 like 2, and 11 like 5. A `n === 1 ? a : b` ternary is
    // wrong for every count in the `few`/`many` rows below.
    it.each([
        [1, 'one'],
        [2, 'few'],
        [4, 'few'],
        [5, 'many'],
        [0, 'many'],
        [11, 'many'],
        [14, 'many'],
        [21, 'one'],
        [22, 'few'],
        [25, 'many'],
        [101, 'one'],
        [111, 'many']
    ])('selects the Russian form %i actually takes (%s)', (count, category) => {
        vi.mocked(hasMessage).mockImplementation((key) => CATEGORY_KEYS.test(key));

        expect(tPlural('days.count', count, 'ru')).toBe(`days.count.${category}`);
    });

    it.each([
        [0, 'other'],
        [1, 'one'],
        [2, 'other'],
        [11, 'other'],
        [21, 'other']
    ])('selects the English form for %i, where only one is singular (%s)', (count, category) => {
        vi.mocked(hasMessage).mockImplementation((key) => CATEGORY_KEYS.test(key));

        expect(tPlural('days.count', count, 'en')).toBe(`days.count.${category}`);
    });

    // A bundle that carries only `other` — or a base key that is missing entirely — must
    // degrade to the general form instead of throwing or printing a category the reader never
    // asked about. The bundles really have no such key, so this runs against the real lookup.
    it('falls back to the .other form when the selected category has no key', () => {
        vi.mocked(hasMessage).mockImplementation((key) => key.endsWith('.other'));

        expect(tPlural('no.such.countedThing', 2, 'ru')).toBe('no.such.countedThing.other');
        expect(hasMessage).toHaveBeenCalledWith('no.such.countedThing.few', 'ru');
    });
});
