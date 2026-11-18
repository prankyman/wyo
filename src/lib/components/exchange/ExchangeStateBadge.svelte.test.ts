import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ExchangeStateBadge from './ExchangeStateBadge.svelte';
import type { TOrderExchange } from './types';

function draw(exchange: TOrderExchange): void {
    render(ExchangeStateBadge, { props: { lang: 'ru', exchange, testId: 'exchange-state' } });
}

/** The exchange's own state of an order, in the tone of the state, with its «?» and, when it has one, the reason or the attention. */
describe('ExchangeStateBadge', () => {
    it.each([
        ['not_sent', 'не передаётся', 'bg-ink-100'],
        ['sent', 'передан', 'bg-wait-50'],
        ['held', 'ждёт у фулфилмент-центра', 'bg-warn-50'],
        ['reserved', 'зарезервирован', 'bg-wait-50'],
        ['label_sent', 'этикетка передана', 'bg-wait-50'],
        ['packed', 'упакован', 'bg-wait-50'],
        ['codes_checking', 'коды проверяются', 'bg-wait-50'],
        ['codes_accepted', 'коды приняты', 'bg-ok-50'],
        ['handed_over', 'передан в доставку', 'bg-ok-50'],
        ['cancelled', 'отменён', 'bg-ink-100']
    ] as const)('draws %s as «%s»', (state, label, toneClass) => {
        draw({ state });

        expect(screen.getByTestId('exchange-state')).toHaveTextContent(label);
        expect(screen.getByTestId('exchange-state')).toHaveClass(toneClass);
        expect(screen.getByTestId('exchange-state-help-button')).toBeInTheDocument();
    });

    it('draws «проблема» in the stop tone when a problem is present, whatever the state says', () => {
        draw({ state: 'label_sent', problem: { code: 'marketplace_refused', at: '2026-11-18T12:00:00' } });

        expect(screen.getByTestId('exchange-state')).toHaveTextContent('проблема');
        expect(screen.getByTestId('exchange-state')).toHaveClass('bg-stop-50');
    });

    it('puts the reason a not-sent order is not sent into the words of its «?»', () => {
        draw({ state: 'not_sent', reason: 'warehouse_not_matched' });

        expect(screen.getByTestId('exchange-state-help-button').getAttribute('aria-label')).toContain(
            'склад этого заказа не сопоставлен складу фулфилмент-центра'
        );
    });

    it('marks an order that needs the owner in the attention tone and says what to do in the «?»', () => {
        draw({ state: 'sent', attention: 'not_reserved_in_time' });

        expect(screen.getByTestId('exchange-state')).toHaveClass('bg-warn-50');
        expect(screen.getByTestId('exchange-state-help-button').getAttribute('aria-label')).toContain(
            'не зарезервировал товар вовремя'
        );
    });
});
