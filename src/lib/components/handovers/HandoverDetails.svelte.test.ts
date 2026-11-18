import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { handoverDetailsOf, handoverOf, exchangeProblemOf } from '$test/support/external-api';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn(), post: vi.fn(), put: vi.fn() }
}));

import { api } from '$lib/api/api';
import HandoverDetails from './HandoverDetails.svelte';
import type { THandoverDetails } from './types';

const get = vi.mocked(api.get);
const post = vi.mocked(api.post);

afterEach(() => {
    get.mockReset();
    post.mockReset();
});

function draw(details: THandoverDetails, onChanged = vi.fn()): void {
    render(HandoverDetails, { props: { lang: 'ru', details, onChanged } });
}

/** One hand-over in full: its state and steps, its facts, its problem, the shipping point to pick and «Повторить». */
describe('HandoverDetails', () => {
    it('draws the state, the strip of steps, the facts of the hand-over, and nothing the owner cannot act on', () => {
        draw(handoverDetailsOf());

        expect(screen.getByTestId('handover-state')).toHaveTextContent('идёт');
        expect(screen.getByTestId('handover-steps-now')).toHaveTextContent('Сейчас: шаг 3 из 6');
        expect(screen.getByTestId('handover-facts-number')).toHaveTextContent('HO-2026-0042');
        expect(screen.getByTestId('handover-facts-day')).toHaveTextContent('19 нояб. 2026');
        expect(screen.getByTestId('handover-facts-batches')).toHaveTextContent('GI-1001');
        expect(screen.getByTestId('handover-facts-orders')).toHaveTextContent('4455667788, 4455667799');
        expect(screen.getByTestId('handover-facts-boxes')).toHaveTextContent('2');
        for (const absent of [
            'handover-problem',
            'handover-retry',
            'shipping-point',
            'handover-closed',
            'handover-note'
        ]) {
            expect(screen.queryByTestId(absent)).toBeNull();
        }
    });

    it('draws the problem with the marketplace’s words and lets the owner repeat the step when the wyo server says he may', async () => {
        post.mockResolvedValue({ ok: true, data: handoverDetailsOf() });
        const onChanged = vi.fn();
        draw(
            handoverDetailsOf({
                handover: handoverOf({
                    state: 'problem',
                    problem: exchangeProblemOf({ code: 'marketplace_refused', params: { step: 'close' } })
                }),
                retryable: true
            }),
            onChanged
        );

        expect(screen.getByTestId('handover-problem')).toHaveTextContent(
            'Маркетплейс отказал на шаге «закрытие партии».'
        );
        await fireEvent.click(screen.getByTestId('handover-retry'));

        await waitFor(() => expect(onChanged).toHaveBeenCalledOnce());
        expect(post).toHaveBeenCalledExactlyOnceWith('/api/v1/handovers/31/retry', {});
        expect(screen.getByTestId('handover-retry-answer-saved')).toHaveTextContent('Повтор начат');
    });

    it('words the refusal of a repeat that is not possible and does not tell the page', async () => {
        post.mockResolvedValue({ ok: false, status: 409, refusal: { code: 'handover_not_retryable' } });
        const onChanged = vi.fn();
        draw(handoverDetailsOf({ retryable: true }), onChanged);

        await fireEvent.click(screen.getByTestId('handover-retry'));

        await waitFor(() =>
            expect(screen.getByTestId('handover-retry-answer-error')).toHaveTextContent(
                'Повторить передачу сейчас нельзя'
            )
        );
        expect(onChanged).not.toHaveBeenCalled();
    });

    it('draws the picker of the shipping point when the owner must choose one', async () => {
        get.mockResolvedValue({ ok: true, data: { points: [{ id: 'p1', name: 'Склад Коледино' }] } });
        draw(handoverDetailsOf({ needsShippingPoint: true }));

        expect(await screen.findByTestId('shipping-point-box')).toBeInTheDocument();
    });

    it('says that nothing can be undone once the batch is closed', () => {
        draw(handoverDetailsOf({ closed: true }));

        expect(screen.getByTestId('handover-closed')).toHaveTextContent('закрыта на маркетплейсе');
    });

    it('draws four steps and says that boxes are not sent and that a withdrawn request undoes nothing after the approval, for a marketplace with no step of boxes', () => {
        draw(
            handoverDetailsOf({
                handover: handoverOf({ boxes: 2 }),
                steps: [
                    { step: 'batch', state: 'done' },
                    { step: 'move', state: 'now' },
                    { step: 'close', state: 'todo' },
                    { step: 'documents', state: 'todo' }
                ]
            })
        );

        expect(screen.getByTestId('handover-steps-now')).toHaveTextContent('Сейчас: шаг 2 из 4');
        expect(screen.getByTestId('handover-note')).toHaveTextContent(
            'транспортные короба, которые назвал фулфилмент-центр, ему не передаются'
        );
        expect(screen.getByTestId('handover-note')).toHaveTextContent('утверждённую партию изменить нельзя');
    });
});
