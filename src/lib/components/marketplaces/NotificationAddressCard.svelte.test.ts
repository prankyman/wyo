import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MARKETPLACE_CODE, THIRD_MARKETPLACE_CODE } from '$test/support/marketplaces';
import NotificationAddressCard from './NotificationAddressCard.svelte';
import type { TAccountNotifications } from './types';

const copyToClipboard = vi.hoisted(() => vi.fn());
vi.mock('$lib/utils/clipboard', () => ({ copyToClipboard }));

afterEach(() => copyToClipboard.mockReset());

const notificationsOf = (change: Partial<TAccountNotifications> = {}): TAccountNotifications => ({
    path: '/hooks/marketplace/k3x9Q2mZp7',
    state: 'waiting',
    ...change
});

const props = (change: { code?: string; notifications?: Partial<TAccountNotifications> } = {}) => ({
    lang: 'ru' as const,
    code: change.code ?? THIRD_MARKETPLACE_CODE,
    origin: 'https://wyo.example.ru',
    notifications: notificationsOf(change.notifications),
    testId: 'notifications'
});

/** An account's notification address, whether the marketplace's notifications arrive, and the steps that connect them in its cabinet. */
describe('NotificationAddressCard', () => {
    it('shows the whole address — the page’s own origin and the account’s path — with a copy button that copies it', async () => {
        copyToClipboard.mockResolvedValue(true);
        render(NotificationAddressCard, { props: props() });

        expect(screen.getByTestId('notifications-heading')).toHaveTextContent('Уведомления маркетплейса');
        expect(screen.getByTestId('notifications-address')).toHaveTextContent(
            'https://wyo.example.ru/hooks/marketplace/k3x9Q2mZp7'
        );
        await fireEvent.click(screen.getByTestId('notifications-copy'));

        await waitFor(() =>
            expect(copyToClipboard).toHaveBeenCalledExactlyOnceWith(
                'https://wyo.example.ru/hooks/marketplace/k3x9Q2mZp7'
            )
        );
        expect(screen.getByTestId('notifications-copy-state')).toHaveTextContent('Скопировано');
    });

    it('takes the page’s own origin when none is given', () => {
        render(NotificationAddressCard, {
            props: {
                lang: 'ru',
                code: THIRD_MARKETPLACE_CODE,
                notifications: notificationsOf(),
                testId: 'notifications'
            }
        });

        expect(screen.getByTestId('notifications-address')).toHaveTextContent(
            `${window.location.origin}/hooks/marketplace/k3x9Q2mZp7`
        );
    });

    it.each([
        ['waiting', 'Уведомления не подключены', 'bg-wait-50'],
        ['working', 'Уведомления подключены', 'bg-ok-50'],
        ['lost', 'Уведомления не доходят', 'bg-warn-50']
    ] as const)('says «%s» as the badge «%s», with a «?» that explains it', (state, label, toneClass) => {
        render(NotificationAddressCard, { props: props({ notifications: { state } }) });

        expect(screen.getByTestId('notifications-state')).toHaveTextContent(label);
        expect(screen.getByTestId('notifications-state')).toHaveClass(toneClass);
        expect(screen.getByTestId('notifications-state-help-button')).toBeInTheDocument();
    });

    it('says what to check in the cabinet and how often wyo reads meanwhile when the notifications do not arrive', () => {
        render(NotificationAddressCard, {
            props: props({
                notifications: { state: 'lost', lostAt: '2026-11-18T12:30:00', seenAt: '2026-11-18T09:00:00' }
            })
        });

        const note = screen.getByTestId('notifications-lost');
        expect(note).toHaveTextContent('18 нояб. 2026');
        expect(note).toHaveTextContent('каждые 5 минут');
        expect(note).toHaveTextContent('каждые 30 минут');
        expect(note).toHaveTextContent('API-уведомления');
        expect(screen.getByTestId('notifications-facts-seenAt')).toHaveTextContent('18 нояб. 2026');
    });

    it('draws no note while the notifications wait or work, and no last-notification fact before the first one', () => {
        render(NotificationAddressCard, { props: props() });

        expect(screen.queryByTestId('notifications-lost')).toBeNull();
        expect(screen.queryByTestId('notifications-facts-seenAt')).toBeNull();
    });

    it('draws the marketplace’s steps in order, open while the notifications wait, folded once they work', () => {
        const { unmount } = render(NotificationAddressCard, { props: props() });

        expect((screen.getByTestId('notifications-steps') as HTMLDetailsElement).open).toBe(true);
        expect(screen.getByTestId('notifications-steps-intro')).toHaveTextContent('в течение нескольких секунд');
        const steps = within(screen.getByTestId('notifications-steps-list')).getAllByRole('listitem');
        expect(steps).toHaveLength(7);
        expect(steps[0]).toHaveTextContent('API-уведомления');
        expect(steps[3]).toHaveTextContent('создание заказа');
        expect(steps[3]).toHaveTextContent('отмена заказа');
        expect(steps[4]).toHaveTextContent('https');
        unmount();

        render(NotificationAddressCard, { props: props({ notifications: { state: 'working' } }) });
        expect((screen.getByTestId('notifications-steps') as HTMLDetailsElement).open).toBe(false);
    });

    it('draws no steps and no cabinet advice for a marketplace the interface has no steps for, but still the address and the state', () => {
        render(NotificationAddressCard, { props: props({ code: MARKETPLACE_CODE, notifications: { state: 'lost' } }) });

        expect(screen.queryByTestId('notifications-steps')).toBeNull();
        expect(screen.getByTestId('notifications-address')).toBeInTheDocument();
        expect(screen.getByTestId('notifications-state')).toHaveTextContent('Уведомления не доходят');
        expect(screen.getByTestId('notifications-lost')).toBeInTheDocument();
    });
});
