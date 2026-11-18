import { render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import AccountStateBadge from './AccountStateBadge.svelte';

beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-11-18T12:00:00'));
});
afterEach(() => vi.useRealTimers());

/** The state of a marketplace account: a badge in the tone of the state, and a «?» that says what it means and what to do. */
describe('AccountStateBadge', () => {
    it.each([
        ['working', undefined, 'Работает', 'bg-ok-50', 'Ничего делать не нужно'],
        ['key_expiring', '2026-11-21T12:00:00', 'Ключ истекает через 3 дня', 'bg-warn-50', 'Заменить ключ'],
        ['key_expiring', '2026-12-02T12:00:00', 'Ключ истекает через 14 дней', 'bg-warn-50', 'за 14 дней'],
        ['key_expiring', undefined, 'Ключ скоро истекает', 'bg-warn-50', 'Заменить ключ'],
        ['key_expired', '2026-11-17T12:00:00', 'Ключ истёк', 'bg-stop-50', 'больше не принимает'],
        ['key_refused', undefined, 'Ключ не принят', 'bg-stop-50', 'кодом 401'],
        ['no_connection', undefined, 'Нет связи', 'bg-stop-50', 'Пять обращений подряд'],
        ['disabled', undefined, 'Отключён', 'bg-ink-100', 'Нажмите «Включить»']
    ] as const)('draws %s (key ends %s) as «%s»', (state, keyExpiresAt, label, toneClass, help) => {
        render(AccountStateBadge, { props: { lang: 'ru', state, keyExpiresAt, testId: 'account-state' } });

        expect(screen.getByTestId('account-state')).toHaveTextContent(label);
        expect(screen.getByTestId('account-state')).toHaveClass(toneClass);
        expect(screen.getByTestId('account-state-help-button').getAttribute('aria-label')).toContain(help);
    });
});
