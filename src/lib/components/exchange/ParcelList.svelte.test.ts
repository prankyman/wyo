import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { parcelOf } from '$test/support/external-api';
import ParcelList from './ParcelList.svelte';
import type { TOrderParcel } from './types';

const ORDER_NUMBER = '4455667788';

function draw(parcels: TOrderParcel[]) {
    return render(ParcelList, { props: { lang: 'ru', parcels, orderNumber: ORDER_NUMBER } });
}

/** An order's parcels at the fulfilment centre: the marketplace's number of each part and the state of its label. */
describe('ParcelList', () => {
    it('draws nothing while the order has no parcel, because the fulfilment centre has not finished packing', () => {
        const { container } = draw([]);

        // Svelte leaves a comment as its anchor; nothing else is drawn.
        expect(container.querySelectorAll('*')).toHaveLength(0);
        expect(container.textContent).toBe('');
    });

    it('draws one parcel whose number is the order’s own as «the same number», with its label not yet attached and no position label', () => {
        draw([parcelOf({ marketplaceNumber: ORDER_NUMBER })]);

        expect(screen.getByTestId('parcels-help-button')).toBeInTheDocument();
        expect(screen.getByText('Посылки заказа')).toBeInTheDocument();
        expect(screen.queryByTestId('parcels-1-position')).toBeNull();
        expect(screen.getByTestId('parcels-1-marketplaceNumber')).toHaveTextContent(
            '4455667788 — тот же номер, что у самого заказа'
        );
        expect(screen.getByTestId('parcels-1-label')).toHaveTextContent('ещё не передана');
    });

    it('says in words that the marketplace has not given a part its number yet, never an empty value', () => {
        draw([parcelOf({ marketplaceNumber: '' })]);

        expect(screen.getByTestId('parcels-1-marketplaceNumber')).toHaveTextContent('ещё нет');
    });

    it('numbers the parcels of an order packed in several, each with the marketplace’s number of its own part and its own label state', () => {
        draw([
            parcelOf({
                number: 1,
                marketplaceNumber: '4455667788-1',
                labelSent: true,
                labelSentAt: '2026-11-18T12:30:00'
            }),
            parcelOf({ number: 2, marketplaceNumber: '4455667788-2', labelSent: false })
        ]);

        expect(screen.getByTestId('parcels-1-position')).toHaveTextContent('Посылка 1 из 2');
        expect(screen.getByTestId('parcels-2-position')).toHaveTextContent('Посылка 2 из 2');
        expect(screen.getByTestId('parcels-1-marketplaceNumber')).toHaveTextContent('4455667788-1');
        expect(screen.getByTestId('parcels-1-marketplaceNumber')).not.toHaveTextContent('тот же номер');
        expect(screen.getByTestId('parcels-2-marketplaceNumber')).toHaveTextContent('4455667788-2');
        expect(screen.getByTestId('parcels-1-label')).toHaveTextContent('передана фулфилмент-центру 18 нояб. 2026');
        expect(screen.getByTestId('parcels-2-label')).toHaveTextContent('ещё не передана');
    });

    it('says a label was attached even when the wyo server did not give the moment', () => {
        draw([parcelOf({ marketplaceNumber: ORDER_NUMBER, labelSent: true })]);

        expect(screen.getByTestId('parcels-1-label')).toHaveTextContent('передана фулфилмент-центру');
        expect(screen.getByTestId('parcels-1-label')).not.toHaveTextContent('2026');
    });
});
