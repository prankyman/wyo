import { describe, expect, it } from 'vitest';
import { orderOf } from '$test/support/marketplaces';
import { productText } from './product-text';

describe('the product of an order in one line', () => {
    it('names the kind, the brand, the size, the article and the first barcode, in this order', () => {
        expect(productText(orderOf(), 'ru')).toBe(
            'Платья · Северный ветер · размер 42 · артикул DRESS-RED-42 · штрихкод 2040000000017'
        );
    });

    it('leaves out what the marketplace did not give, and is empty when it gave nothing', () => {
        expect(productText(orderOf({ kind: undefined, brand: undefined, size: undefined, barcodes: [] }), 'ru')).toBe(
            'артикул DRESS-RED-42'
        );
        expect(
            productText(
                orderOf({ kind: undefined, brand: undefined, size: undefined, article: undefined, barcodes: [] }),
                'ru'
            )
        ).toBe('');
    });
});
