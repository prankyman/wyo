import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { INotificationLine } from '$lib/notifications/types';
import NotificationPanel from './NotificationPanel.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));

const line = (
    key: string,
    urgency: INotificationLine['urgency'],
    href: string | null = `/settings#${key}`
): INotificationLine => ({ key, kind: key, urgency, text: `Строка ${key}`, href });

const props = (lines: INotificationLine[]) => ({ lang: 'ru' as const, lines, onNavigate: vi.fn() });

describe('NotificationPanel', () => {
    it('teaches what the bell will do when nothing waits', () => {
        render(NotificationPanel, { props: props([]) });

        expect(screen.getByTestId('notification-panel-empty')).toHaveTextContent(
            'Сейчас ничего не ждёт вашего внимания. Когда что-то появится, колокольчик станет жёлтым, а срочное — красным.'
        );
        expect(screen.queryByTestId('notification-lines')).toBeNull();
    });

    it('lists the lines, marks an urgent one with a warning word, links a line and leaves a line without a page as text', async () => {
        const view = props([line('a', 'urgent'), line('b', 'attention', null)]);
        render(NotificationPanel, { props: view });

        const [urgent, plain] = screen.getAllByTestId('notification-line');
        expect(urgent).toHaveTextContent('Срочно:');
        expect(urgent).toHaveAttribute('data-urgency', 'urgent');
        expect(urgent).toHaveAttribute('data-kind', 'a');
        expect(plain).toHaveTextContent('Строка b');
        expect(plain.querySelector('a')).toBeNull();

        expect(screen.getByTestId('notification-line-link')).toHaveAttribute('href', '/settings#a');
        await fireEvent.click(screen.getByTestId('notification-line-link'));
        expect(view.onNavigate).toHaveBeenCalledOnce();
        expect(screen.queryByTestId('notification-panel-empty')).toBeNull();
    });
});
