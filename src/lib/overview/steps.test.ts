import { describe, expect, it } from 'vitest';
import { overviewSteps } from './steps';

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
