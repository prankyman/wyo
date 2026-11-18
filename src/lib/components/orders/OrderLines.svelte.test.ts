import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { lineOf } from '$test/support/marketplaces';
import OrderLines from './OrderLines.svelte';

/** An order's products with how many units of each, for the details beside the order list. */
describe('OrderLines', () => {
    it('draws the one product of an order with every fact it has, its quantity in words, and no position label', () => {
        render(OrderLines, { props: { lang: 'ru', lines: [lineOf()] } });

        expect(screen.getByTestId('order-lines-help-button')).toBeInTheDocument();
        expect(screen.getByText('Товары заказа')).toBeInTheDocument();
        expect(screen.getByTestId('order-lines-1-article')).toHaveTextContent('DRESS-RED-42');
        expect(screen.getByTestId('order-lines-1-barcodes')).toHaveTextContent('2040000000017');
        expect(screen.getByTestId('order-lines-1-productId')).toHaveTextContent('987654');
        expect(screen.getByTestId('order-lines-1-kind')).toHaveTextContent('Платья');
        expect(screen.getByTestId('order-lines-1-brand')).toHaveTextContent('Северный ветер');
        expect(screen.getByTestId('order-lines-1-size')).toHaveTextContent('42');
        expect(screen.getByTestId('order-lines-1-quantity')).toHaveTextContent('1 единица товара');
        expect(screen.queryByTestId('order-lines-1-name')).toBeNull();
        expect(screen.queryByTestId('order-lines-1-position')).toBeNull();
    });

    it('draws each product of an order of several as its own block, numbered, each with its own quantity', () => {
        render(OrderLines, {
            props: {
                lang: 'ru',
                lines: [
                    lineOf({ name: 'Платье красное', quantity: 3, barcodes: ['2040000000017', '2040000000024'] }),
                    lineOf({ article: 'SHIRT-BLUE-M', name: 'Рубашка синяя', quantity: 5, barcodes: [] }),
                    lineOf({ article: 'HAT-GREY', quantity: 2, kind: undefined, brand: undefined, size: undefined })
                ]
            }
        });

        expect(screen.getByTestId('order-lines-1-position')).toHaveTextContent('Позиция 1 из 3');
        expect(screen.getByTestId('order-lines-3-position')).toHaveTextContent('Позиция 3 из 3');
        expect(screen.getByTestId('order-lines-1-name')).toHaveTextContent('Платье красное');
        expect(screen.getByTestId('order-lines-1-barcodes')).toHaveTextContent('2040000000017, 2040000000024');
        expect(screen.getByTestId('order-lines-1-quantity')).toHaveTextContent('3 единицы товара');
        expect(screen.getByTestId('order-lines-2-article')).toHaveTextContent('SHIRT-BLUE-M');
        expect(screen.getByTestId('order-lines-2-quantity')).toHaveTextContent('5 единиц товара');
        expect(screen.getByTestId('order-lines-3-quantity')).toHaveTextContent('2 единицы товара');
    });

    it('leaves out a fact the marketplace did not give, never drawing it empty', () => {
        render(OrderLines, {
            props: {
                lang: 'ru',
                lines: [
                    lineOf({
                        productId: undefined,
                        article: undefined,
                        kind: undefined,
                        brand: undefined,
                        size: undefined,
                        barcodes: []
                    })
                ]
            }
        });

        expect(screen.getByTestId('order-lines-1-quantity')).toBeInTheDocument();
        for (const fact of ['name', 'article', 'barcodes', 'productId', 'kind', 'brand', 'size']) {
            expect(screen.queryByTestId(`order-lines-1-${fact}`), fact).toBeNull();
        }
    });
});
