import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { INotificationLine } from '$lib/notifications/types';
import NotificationBell from './NotificationBell.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

const line = (key: string, urgency: INotificationLine['urgency']): INotificationLine => ({
    key,
    kind: key,
    urgency,
    text: `Строка ${key}`,
    href: `/settings#${key}`
});

const props = (lines: INotificationLine[] = []) => ({ lang: 'ru' as const, lines, helpText: 'Помощь по уведомлениям' });

const bell = () => screen.getByTestId('notification-bell');

afterEach(() => {
    vi.unstubAllGlobals();
    window.innerWidth = 1024;
});

describe('NotificationBell — how it looks', () => {
    it.each([
        ['nothing', [], 'quiet', null, 'Уведомления: ничего нет'],
        ['information only', [line('a', 'info')], 'attention', '1', 'Уведомления: 1, срочных: 0'],
        [
            'two urgent lines and a third',
            [line('a', 'urgent'), line('b', 'urgent'), line('c', 'attention')],
            'urgent',
            '3',
            'Уведомления: 3, срочных: 2'
        ]
    ] as const)('%s: tone %s, number %s, name «%s»', (_case, lines, tone, number, name) => {
        render(NotificationBell, { props: props([...lines]) });

        expect(bell()).toHaveAttribute('data-tone', tone);
        expect(bell()).toHaveAccessibleName(name);
        const shown = screen.queryByTestId('notification-bell-number');
        if (number === null) expect(shown).toBeNull();
        else expect(shown).toHaveTextContent(number);
    });

    it('shows «99+» above 99', () => {
        const lines = Array.from({ length: 120 }, (_, index) => line(`k${index}`, 'info'));
        render(NotificationBell, { props: props(lines) });

        expect(screen.getByTestId('notification-bell-number')).toHaveTextContent('99+');
    });
});

describe('NotificationBell — the pulse and the live region', () => {
    it('pulses for an unseen urgent line, stops when the panel opens, and speaks a line that arrives later', async () => {
        const view = render(NotificationBell, { props: props([line('a', 'urgent'), line('b', 'attention')]) });
        expect(bell()).toHaveAttribute('data-pulsing', 'true');
        expect(screen.getByTestId('notification-bell-pulse')).toBeInTheDocument();

        await fireEvent.click(bell());
        await fireEvent.click(bell());
        expect(bell()).toHaveAttribute('data-pulsing', 'false');

        await view.rerender(props([line('a', 'urgent'), line('b', 'attention'), line('c', 'urgent')]));
        expect(bell()).toHaveAttribute('data-pulsing', 'true');
        expect(screen.getByTestId('notification-bell-live')).toHaveTextContent('Новое уведомление: Строка c');
    });

    it('never draws the pulse where the device asks for less motion, and says nothing for the first lines', () => {
        vi.stubGlobal(
            'matchMedia',
            vi.fn((query: string) => ({
                matches: query.includes('reduce'),
                media: query,
                addEventListener: vi.fn(),
                removeEventListener: vi.fn()
            }))
        );
        render(NotificationBell, { props: props([line('a', 'urgent')]) });

        expect(bell()).toHaveAttribute('data-tone', 'urgent');
        expect(bell()).toHaveAttribute('data-pulsing', 'false');
        expect(bell()).toHaveAttribute('data-unseen', 'true');
        expect(screen.queryByTestId('notification-bell-pulse')).toBeNull();
        expect(screen.getByTestId('notification-bell-live')).toHaveTextContent('');
    });
});

describe('NotificationBell — the panel', () => {
    it('opens under the bell on a computer with one «?», and a line’s link closes it', async () => {
        render(NotificationBell, { props: props([line('a', 'attention'), line('b', 'info')]) });

        await fireEvent.click(bell());
        const panel = screen.getByTestId('notification-panel');
        expect(panel).toHaveAttribute('data-form', 'popover');
        expect(bell()).toHaveAttribute('aria-expanded', 'true');
        expect(within(panel).getAllByTestId('notification-panel-help-button')).toHaveLength(1);

        await fireEvent.click(within(panel).getAllByTestId('notification-line-link')[0]);
        expect(screen.queryByTestId('notification-panel')).toBeNull();
    });

    it('fills the screen in the phone sheet below 768 px and closes with its «×»', async () => {
        window.innerWidth = 390;
        render(NotificationBell, { props: props([line('a', 'urgent')]) });

        await fireEvent.click(bell());
        expect(screen.getByTestId('notification-panel-panel')).toBeInTheDocument();
        expect(screen.getAllByTestId('notification-panel-help-button')).toHaveLength(1);
        await fireEvent.click(screen.getByTestId('notification-panel-close'));
        expect(screen.queryByTestId('notification-panel-panel')).toBeNull();
        expect(document.activeElement).toBe(bell());
    });

    it('closes on Escape on a computer', async () => {
        render(NotificationBell, { props: props([line('a', 'info')]) });

        await fireEvent.click(bell());
        await fireEvent.keyDown(document, { key: 'Escape' });
        expect(screen.queryByTestId('notification-panel')).toBeNull();
    });
});
