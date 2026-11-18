import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { exchangeDetailsOf, exchangeProblemOf, parcelOf } from '$test/support/external-api';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { post: vi.fn() }
}));

import { api } from '$lib/api/api';
import OrderExchangeDetails from './OrderExchangeDetails.svelte';
import type { TOrderExchangeDetails } from './types';

const post = vi.mocked(api.post);

afterEach(() => {
    post.mockReset();
});

function draw(details: TOrderExchangeDetails, onRetried = vi.fn()): void {
    render(OrderExchangeDetails, {
        props: { lang: 'ru', orderId: 101, orderNumber: '4455667788', details, onRetried }
    });
}

/** One order's exchange with the fulfilment centre in full: state, reason, held reasons, problem, verdicts, cancellation, history. */
describe('OrderExchangeDetails', () => {
    it('draws the state, the number at the fulfilment centre and the history, and nothing for what the order does not have', () => {
        draw(exchangeDetailsOf());

        expect(screen.getByTestId('exchange-state')).toHaveTextContent('зарезервирован');
        expect(screen.getByTestId('exchange-facts-fcNumber')).toHaveTextContent('FC-5501');
        expect(screen.getAllByTestId('exchange-history-row').map((row) => row.textContent)).toEqual([
            expect.stringContaining('Заказ передан фулфилмент-центру'),
            expect.stringContaining('Товар зарезервирован')
        ]);
        for (const absent of [
            'exchange-held',
            'parcels',
            'exchange-problem',
            'exchange-verdicts',
            'exchange-cancel',
            'exchange-retry'
        ]) {
            expect(screen.queryByTestId(absent)).toBeNull();
        }
    });

    it('draws the order’s parcels with the marketplace’s own number of each, from the details it is given', () => {
        draw(
            exchangeDetailsOf({
                parcels: [
                    parcelOf({ number: 1, marketplaceNumber: '4455667788-1', labelSent: true }),
                    parcelOf({ number: 2, marketplaceNumber: '4455667788-2' })
                ]
            })
        );

        expect(screen.getByTestId('parcels-1-marketplaceNumber')).toHaveTextContent('4455667788-1');
        expect(screen.getByTestId('parcels-2-marketplaceNumber')).toHaveTextContent('4455667788-2');
    });

    it('says why an order is not sent and what needs the owner, in sentences', () => {
        draw(exchangeDetailsOf({ state: 'not_sent', reason: 'warehouse_not_matched', attention: 'held' }));

        expect(screen.getByTestId('exchange-reason')).toHaveTextContent('склад этого заказа не сопоставлен');
        expect(screen.getByTestId('exchange-attention')).toHaveTextContent(
            'фулфилмент-центр не может принять его в работу'
        );
    });

    it('draws the held reasons, the problem with the marketplace’s words, the verdicts and the answer to a cancellation', () => {
        draw(
            exchangeDetailsOf({
                state: 'held',
                heldReasons: [{ code: 'warehouse_missing' }],
                problem: exchangeProblemOf(),
                verdicts: [{ identifier: 'marking', verdict: 'refused', words: 'Код не найден' }],
                cancelAnswer: 'needs_person'
            })
        );

        expect(screen.getByTestId('exchange-held')).toHaveTextContent('В заказе не указан склад.');
        expect(screen.getByTestId('exchange-problem')).toHaveTextContent('Маркетплейс отказал на шаге');
        expect(screen.getByTestId('exchange-verdicts')).toHaveTextContent('Код маркировки');
        expect(screen.getByTestId('exchange-cancel')).toHaveTextContent('отменить заказ должен человек на складе');
        expect(screen.getByTestId('exchange-state')).toHaveTextContent('проблема');
    });

    it('presses «Передать заново» through the order’s own call, says so and tells the page to read the order again', async () => {
        post.mockResolvedValue({ ok: true, data: {} });
        const onRetried = vi.fn();
        draw(exchangeDetailsOf({ retryable: true }), onRetried);

        await fireEvent.click(screen.getByTestId('exchange-retry'));

        await waitFor(() => expect(onRetried).toHaveBeenCalledOnce());
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/orders/101/exchange/retry', {});
        expect(screen.getByTestId('exchange-retry-answer-saved')).toHaveTextContent('Повтор начат');
    });

    it('words the refusal of a repeat that is no longer possible and does not tell the page', async () => {
        post.mockResolvedValue({ ok: false, status: 409, refusal: { code: 'exchange_not_retryable' } });
        const onRetried = vi.fn();
        draw(exchangeDetailsOf({ retryable: true }), onRetried);

        await fireEvent.click(screen.getByTestId('exchange-retry'));

        await waitFor(() =>
            expect(screen.getByTestId('exchange-retry-answer-error')).toHaveTextContent(
                'Передать заказ заново сейчас нельзя'
            )
        );
        expect(onRetried).not.toHaveBeenCalled();
    });
});
