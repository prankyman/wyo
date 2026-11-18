import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { handoverDetailsOf } from '$test/support/external-api';
import HandoverSteps from './HandoverSteps.svelte';
import type { THandoverStepState } from './types';

const ORDER = ['batch', 'move', 'boxes', 'shipping', 'close', 'documents'] as const;

/** The steps of a hand-over it is given with their state (six for one marketplace, four for another), and «Сейчас: …» for the one that is running, waiting or failed. */
function draw(states: THandoverStepState['state'][]): void {
    const steps = ORDER.map((step, index) => ({ step, state: states[index] }));
    render(HandoverSteps, { props: { lang: 'ru', steps, dataTest: 'handover-steps' } });
}

describe('HandoverSteps', () => {
    it('draws the six steps in order and marks the running one as current, with «Сейчас: …» naming it', () => {
        render(HandoverSteps, { props: { lang: 'ru', steps: handoverDetailsOf().steps, dataTest: 'handover-steps' } });

        expect(ORDER.map((step) => screen.getByTestId(`handover-steps-${step}`).getAttribute('data-state'))).toEqual([
            'done',
            'done',
            'current',
            'todo',
            'todo',
            'todo'
        ]);
        expect(screen.getByTestId('handover-steps-boxes-label')).toHaveTextContent('Добавление коробов');
        expect(screen.getByTestId('handover-steps-now')).toHaveTextContent('Сейчас: шаг 3 из 6 — добавление коробов.');
    });

    it.each([
        [
            'waits for the owner',
            ['done', 'done', 'done', 'waiting', 'todo', 'todo'],
            'Сейчас: шаг 4 из 6 — указание способа и пункта сдачи; он ждёт вашего решения.',
            'ждёт'
        ],
        [
            'failed',
            ['done', 'done', 'done', 'done', 'failed', 'todo'],
            'Сейчас: шаг 5 из 6 — закрытие партии; он не удался.',
            'не удался'
        ]
    ] as const)(
        'says which step %s in the words of the step and in the note of the "now" line',
        (_case, states, now, mark) => {
            draw([...states]);

            expect(screen.getByTestId('handover-steps-now')).toHaveTextContent(now);
            const current = screen
                .getAllByRole('listitem')
                .find((item) => item.getAttribute('data-state') === 'current')!;
            expect(current).toHaveTextContent(mark);
        }
    );

    it.each([
        ['every step is done', ['done', 'done', 'done', 'done', 'done', 'done'], 'Сейчас: все шаги выполнены.'],
        ['no step has begun', ['todo', 'todo', 'todo', 'todo', 'todo', 'todo'], 'Сейчас: передача ещё не началась.']
    ] as const)('says when %s', (_case, states, now) => {
        draw([...states]);

        expect(screen.getByTestId('handover-steps-now')).toHaveTextContent(now);
    });

    it('draws exactly the four steps of a marketplace that declares four, numbers them out of four, and draws none of the others', () => {
        const steps: THandoverStepState[] = [
            { step: 'batch', state: 'done' },
            { step: 'move', state: 'failed' },
            { step: 'close', state: 'todo' },
            { step: 'documents', state: 'todo' }
        ];
        render(HandoverSteps, { props: { lang: 'ru', steps, dataTest: 'handover-steps' } });

        expect(screen.getAllByRole('listitem').map((item) => item.getAttribute('data-test'))).toEqual([
            'handover-steps-batch',
            'handover-steps-move',
            'handover-steps-close',
            'handover-steps-documents'
        ]);
        expect(screen.queryByTestId('handover-steps-boxes')).toBeNull();
        expect(screen.queryByTestId('handover-steps-shipping')).toBeNull();
        expect(screen.getByTestId('handover-steps-now')).toHaveTextContent(
            'Сейчас: шаг 2 из 4 — перенос заказов в партию; он не удался.'
        );
    });
});
