import { describe, expect, it } from 'vitest';
import { THIRD_MARKETPLACE_CODE } from '$test/support/marketplaces';
import { batchKindOf } from './batch-kind';

/** The kind of a batch the marketplace makes by itself, worded from the marketplace's own family of texts. */
describe('batchKindOf', () => {
    it('is nothing for a hand-over whose batches wyo made itself', () => {
        expect(batchKindOf({ marketplace: THIRD_MARKETPLACE_CODE }, 'ru')).toBeNull();
    });

    it.each([
        ['WITHDRAW', 'Маркет забирает со склада', 'скачайте её сами в личном кабинете'],
        ['IMPORT', 'Склад везёт в сортировочный центр или пункт приёма', 'паллеты']
    ])('words the kind %s in the marketplace’s family, with what the owner does about it', (word, text, help) => {
        const kind = batchKindOf({ marketplace: THIRD_MARKETPLACE_CODE, batchKind: word }, 'ru');

        expect(kind?.text).toBe(text);
        expect(kind?.help).toContain(help);
    });

    it('gives a word wyo has no text for as the marketplace sent it, with no «?»', () => {
        expect(batchKindOf({ marketplace: THIRD_MARKETPLACE_CODE, batchKind: 'SOMETHING_NEW' }, 'ru')).toEqual({
            text: 'SOMETHING_NEW'
        });
    });
});
