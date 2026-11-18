import { describe, expect, it } from 'vitest';
import { externalApiCard, type TExternalApiOverview } from './external-api-card';

const WORKING: TExternalApiOverview = {
    state: 'working',
    lastEventAt: '2026-11-18T12:00:00',
    attention: 0,
    handoversWaiting: 0
};

describe('externalApiCard', () => {
    it('draws one card with a title, a help and an action to the page «Внешний API»', () => {
        const card = externalApiCard(WORKING, 'ru');

        expect([card.key, card.title, card.action.href]).toEqual(['externalApi', 'Внешний API', '/external-api']);
        expect(card.help.length).toBeGreaterThan(80);
        expect(card.moreActions).toEqual([]);
    });

    it.each([
        ['empty', 'off', 'Не заполнено', 'wyo ничего не передаёт фулфилмент-центру'],
        ['working', 'ok', 'Работает', 'Последнее событие от фулфилмент-центра: 18 нояб. 2026'],
        ['key_refused', 'stop', 'Ключ не принят', 'Последнее событие'],
        ['read_only_key', 'stop', 'Ключ только читает', 'Последнее событие'],
        ['api_off', 'stop', 'API выключен', 'Последнее событие'],
        ['forbidden', 'stop', 'Доступ запрещён', 'Последнее событие'],
        ['unavailable', 'warn', 'Система недоступна', 'Последнее событие'],
        ['no_connection', 'warn', 'Нет связи', 'Последнее событие'],
        ['unreadable', 'warn', 'Ответ не понят', 'Последнее событие']
    ] as const)('says the state %s in its tone and words', (state, tone, label, sentence) => {
        const card = externalApiCard({ ...WORKING, state }, 'ru');

        expect(card.state).toEqual({ label, tone });
        expect(card.sentence).toContain(sentence);
    });

    it('says when no event came yet', () => {
        const card = externalApiCard({ ...WORKING, lastEventAt: undefined }, 'ru');

        expect(card.sentence).toContain('Событий от фулфилмент-центра ещё не было');
    });

    it('counts the orders that need the owner with a link to that filter, and the hand-overs that wait with a link to their page', () => {
        const card = externalApiCard({ ...WORKING, attention: 3, handoversWaiting: 1 }, 'ru');

        expect(card.sentence).toContain('Заказов, которым нужно ваше решение: 3.');
        expect(card.sentence).toContain('Передач, которые ждут или остановились: 1.');
        expect(card.moreActions).toEqual([
            { label: 'Показать заказы, которым нужно ваше решение (3)', href: '/orders?attention=true' },
            { label: 'Открыть «Передачи»', href: '/handovers' }
        ]);
    });
});
