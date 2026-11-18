import { describe, expect, it } from 'vitest';
import { marketplaceCards, type TMarketplacesOverview } from './marketplace-cards';

const WORKING: TMarketplacesOverview = {
    accounts: 2,
    attention: 0,
    newOrders: 3,
    assembling: 1,
    overdue: 0,
    unknownStates: 0,
    lastSuccessAt: '2026-11-18T12:00:00'
};

const cardOf = (overview: TMarketplacesOverview, key: string) =>
    marketplaceCards(overview, 'ru').find((card) => card.key === key)!;

describe('marketplaceCards', () => {
    it('draws the accounts, the orders and the last reading, each with a help and an action to its page', () => {
        const cards = marketplaceCards(WORKING, 'ru');

        expect(cards.map((card) => [card.key, card.title, card.action.href])).toEqual([
            ['accounts', 'Аккаунты маркетплейсов', '/marketplaces'],
            ['orders', 'Заказы', '/orders'],
            ['reading', 'Последнее чтение', '/journal']
        ]);
        for (const card of cards) expect(card.help.length).toBeGreaterThan(80);
    });

    it.each([
        ['no account yet', { accounts: 0 }, 'warn', 'Нет аккаунтов', 'Добавьте первый аккаунт'],
        ['every account working', {}, 'ok', 'Работают', 'Подключено аккаунтов: 2, все работают.'],
        ['an account needing attention', { attention: 1 }, 'stop', 'Требуют внимания', 'нужно ваше внимание: 1 из 2']
    ])('says how the accounts stand: %s', (_case, change, tone, label, sentence) => {
        const card = cardOf({ ...WORKING, ...change }, 'accounts');

        expect(card.state).toEqual({ label, tone });
        expect(card.sentence).toContain(sentence);
    });

    it.each([
        ['nothing overdue', {}, 'ok', 'Новых заказов: 3, на сборке: 1, просроченных нет.'],
        ['an order past its deadline', { overdue: 2 }, 'stop', 'Просрочено заказов: 2. Новых: 3, на сборке: 1.'],
        ['an order in a state wyo does not know', { unknownStates: 4 }, 'ok', 'Заказов в неизвестном состоянии: 4']
    ])('counts the orders: %s', (_case, change, tone, sentence) => {
        const card = cardOf({ ...WORKING, ...change }, 'orders');

        expect(card.state.tone).toBe(tone);
        expect(card.sentence).toContain(sentence);
    });

    it('says when the last successful reading was, or that there was none yet', () => {
        expect(cardOf(WORKING, 'reading').state).toEqual({ label: 'Было', tone: 'ok' });
        expect(cardOf(WORKING, 'reading').sentence).toContain('18 нояб. 2026');
        expect(cardOf({ ...WORKING, lastSuccessAt: undefined }, 'reading').state).toEqual({
            label: 'Ещё не было',
            tone: 'wait'
        });
    });
});
