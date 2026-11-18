import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import GaugeIcon from 'phosphor-svelte/lib/GaugeIcon';
import type { INavItem } from '$lib/nav/nav-items';
import AppNav from './AppNav.svelte';
import { NavGroupsState } from '$lib/nav/nav-groups.svelte';

/**
 * The menu as the shell draws it: the same list as a left column and as a strip along the bottom of a phone. jsdom applies no
 * media query, so each layout is rendered on its own (`variant`) and told apart by its markup.
 */

function item(testId: string, href: string, extra: Partial<INavItem> = {}): INavItem {
    return { labelKey: `x.${testId}`, href, testId, icon: GaugeIcon, ...extra };
}

/** A made-up menu: seven items, so the strip is full and «Ещё» holds the rest, and one group with two items. */
const MANY: INavItem[] = [
    ...Array.from({ length: 5 }, (_, index) => item(`nav-p${index}`, `/p${index}`, { phoneTab: index + 1 })),
    item('nav-a', '/a', { section: 'tools' }),
    item('nav-b', '/b', { section: 'tools' })
];
const MANY_PROPS = { sections: ['tools'], sectionLabelKeys: { tools: 'x.group' }, version: null };
const PROPS = { lang: 'ru' as const, currentPath: '/settings', version: null };

describe('AppNav, the left column', () => {
    it('draws every item of the menu as a link, the open page marked, and the version line when it is given', () => {
        render(AppNav, { props: { ...PROPS, variant: 'sidebar', version: '1.2.3' } });

        const overview = screen.getByTestId('nav-overview');
        const settings = screen.getByTestId('nav-settings');
        expect(overview).toHaveAttribute('href', '/');
        expect(overview).not.toHaveAttribute('aria-current');
        expect(settings).toHaveAttribute('href', '/settings');
        expect(settings).toHaveAttribute('aria-current', 'page');
        expect(settings).toHaveTextContent('Настройки');
        expect(screen.getByTestId('nav-version')).toHaveTextContent('Версия wyo: 1.2.3');
        expect(screen.queryByTestId('app-tabstrip')).toBeNull();
    });

    it('draws a counter beside an item that has a count, with its sentence, and none for a count of 0', () => {
        render(AppNav, {
            props: {
                ...PROPS,
                variant: 'sidebar',
                counters: {
                    'nav-settings': { count: 2, sentence: 'Two things wait' },
                    'nav-overview': { count: 0, sentence: 'Nothing waits' }
                }
            }
        });

        expect(screen.getByTestId('nav-settings-count')).toHaveAttribute('aria-label', 'Two things wait');
        expect(screen.queryByTestId('nav-overview-count')).toBeNull();
    });

    it('folds items into groups that open and close by hand, a closed group showing the sum of its counters', async () => {
        const groups = new NavGroupsState('/p0', MANY, ['tools']);
        render(AppNav, {
            props: {
                ...PROPS,
                ...MANY_PROPS,
                currentPath: '/p0',
                items: MANY,
                variant: 'sidebar',
                groups,
                counters: { 'nav-a': { count: 3, sentence: 'Three in A' } }
            }
        });

        const group = screen.getByTestId('nav-group-tools');
        expect(within(group).getByTestId('nav-group-tools-heading')).toHaveTextContent('x.group');
        expect(group).not.toHaveAttribute('open');
        expect(screen.getByTestId('nav-group-tools-count')).toHaveTextContent('3');

        groups.setOpen('tools', true);
        await fireEvent(group, new Event('toggle'));
        expect(groups.isOpen('tools')).toBe(true);
        expect(screen.queryByTestId('nav-group-tools-count')).toBeNull();
        expect(within(group).getByTestId('nav-a')).toHaveAttribute('href', '/a');
    });
});

describe('AppNav, the strip along the bottom of a phone', () => {
    it('draws the items as tabs, marks the open page, and shows «Ещё» only when it has something to hold', () => {
        const { unmount } = render(AppNav, { props: { ...PROPS, variant: 'tabstrip' } });

        expect(screen.getByTestId('nav-overview-tab')).toHaveAttribute('href', '/');
        expect(screen.getByTestId('nav-settings-tab')).toHaveAttribute('aria-current', 'page');
        expect(screen.queryByTestId('nav-more-tab')).toBeNull();
        expect(screen.queryByTestId('app-sidebar')).toBeNull();
        unmount();

        render(AppNav, { props: { ...PROPS, variant: 'tabstrip', version: '1.2.3' } });
        expect(screen.getByTestId('nav-more-tab')).toBeInTheDocument();
    });

    it('puts the items past the five tabs in the «Ещё» sheet, with their group, and closes the sheet on a choice', async () => {
        render(AppNav, { props: { ...PROPS, ...MANY_PROPS, currentPath: '/b', items: MANY, variant: 'tabstrip' } });

        expect(screen.getAllByTestId('nav-tab-label')).toHaveLength(6);
        expect(screen.queryByTestId('nav-a-more')).toBeNull();

        await fireEvent.click(screen.getByTestId('nav-more-tab'));
        expect(screen.getByTestId('nav-more-group-tools')).toHaveAttribute('open');
        expect(screen.getByTestId('nav-b-more')).toHaveAttribute('aria-current', 'page');
        expect(screen.getByTestId('nav-a-more')).toHaveAttribute('href', '/a');

        await fireEvent.click(screen.getByTestId('nav-a-more'));
        expect(screen.queryByTestId('nav-a-more')).toBeNull();
    });
});
