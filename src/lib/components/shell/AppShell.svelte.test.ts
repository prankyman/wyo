import { render, screen, within } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import type { INotificationLine } from '$lib/notifications/types';
import AppShell from './AppShell.svelte';

const children = createRawSnippet(() => ({ render: () => '<p data-test="page-body">страница</p>' }));
const LINE: INotificationLine = {
    key: 'a',
    kind: 'backup_none',
    urgency: 'attention',
    text: 'Нет копий',
    href: '/settings'
};

const props = (change: Partial<{ currentPath: string; lines: INotificationLine[]; version: string | null }> = {}) => ({
    lang: 'ru' as const,
    login: 'owner',
    currentPath: '/',
    lines: [],
    version: '1.2.3',
    children,
    ...change
});

describe('AppShell', () => {
    it('draws the top bar with the program’s name, the bell and the owner’s login, and the page between the menus', () => {
        render(AppShell, { props: props({ lines: [LINE] }) });

        const header = screen.getByTestId('app-header');
        expect(within(header).getByTestId('app-brand')).toHaveTextContent('wyo');
        expect(within(header).getByTestId('app-brand')).toHaveAttribute('href', '/');
        expect(within(header).getByTestId('notification-bell')).toHaveAttribute('data-tone', 'attention');
        expect(within(header).getByTestId('app-user-login')).toHaveAttribute('href', '/account');
        expect(within(header).getByTestId('app-user-login')).toHaveAccessibleName('owner');
        expect(within(screen.getByTestId('app-main')).getByTestId('page-body')).toBeInTheDocument();
    });

    it('draws the menu twice from one list, as a column and as a strip, both marking the open page and showing the version', () => {
        render(AppShell, { props: props({ currentPath: '/settings' }) });

        expect(within(screen.getByTestId('app-sidebar')).getByTestId('nav-settings')).toHaveAttribute(
            'aria-current',
            'page'
        );
        expect(within(screen.getByTestId('app-tabstrip')).getByTestId('nav-settings-tab')).toHaveAttribute(
            'aria-current',
            'page'
        );
        expect(screen.getByTestId('nav-version')).toHaveTextContent('Версия wyo: 1.2.3');
    });

    it('draws no version line before the version is known, and nothing of the language switch while one language exists', () => {
        render(AppShell, { props: props({ version: null }) });

        expect(screen.queryByTestId('nav-version')).toBeNull();
        expect(screen.queryByTestId('language-switcher')).toBeNull();
    });
});
