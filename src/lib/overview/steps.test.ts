import { describe, expect, it } from 'vitest';
import { currentStep, overviewSteps } from './steps';

describe('overviewSteps', () => {
    it('lists the four steps of the seller in order, the installation done and the keys of the marketplaces current', () => {
        const { steps, now } = overviewSteps('ru');

        expect(steps.map((step) => [step.key, step.label, step.state])).toEqual([
            ['install', 'Установка', 'done'],
            ['keys', 'Ключи маркетплейсов', 'current'],
            ['orders', 'Заказы', 'todo'],
            ['api', 'Внешний API (по желанию)', 'todo']
        ]);
        expect(now).toBe('Сейчас: шаг 2 — ключи маркетплейсов.');
    });
});

describe('currentStep', () => {
    it.each([
        ['no account yet', 0, 'empty', 'keys'],
        ['accounts and the external API still empty', 1, 'empty', 'orders'],
        ['accounts and the external API filled and working', 2, 'working', 'api'],
        ['accounts and the external API filled but refused', 1, 'key_refused', 'api'],
        ['the external API filled before any account', 0, 'working', 'keys']
    ] as const)('stands at the step for %s', (_case, accounts, state, step) => {
        expect(currentStep(accounts, state)).toBe(step);
    });
});
