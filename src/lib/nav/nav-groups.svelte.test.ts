import { describe, expect, it } from 'vitest';
import { NavGroupsState } from './nav-groups.svelte';
import type { INavItem, TNavSection } from './types';

const ICON = (() => {}) as unknown as INavItem['icon'];
const ITEMS: INavItem[] = [
    { labelKey: 'a', href: '/', testId: 'nav-a', icon: ICON },
    { labelKey: 'b', href: '/orders', testId: 'nav-b', icon: ICON, section: 'orders' as TNavSection },
    { labelKey: 'c', href: '/settings', testId: 'nav-c', icon: ICON, section: 'settings' as TNavSection }
];
const SECTIONS: readonly TNavSection[] = ['orders', 'settings'];

describe('NavGroupsState', () => {
    it('opens the group of the page the visit starts on, and only that one', () => {
        const state = new NavGroupsState('/settings', ITEMS, SECTIONS);

        expect(state.isOpen('settings')).toBe(true);
        expect(state.isOpen('orders')).toBe(false);
    });

    it('opens the first group on the home page', () => {
        expect(new NavGroupsState('/', ITEMS, SECTIONS).isOpen('orders')).toBe(true);
    });

    it('opens and closes a group by hand, and follows a page change by opening that page’s group and closing nothing', () => {
        const state = new NavGroupsState('/settings', ITEMS, SECTIONS);

        state.follow('/orders', ITEMS);
        expect(state.isOpen('orders')).toBe(true);
        expect(state.isOpen('settings')).toBe(true);

        state.setOpen('settings', false);
        state.follow('/orders', ITEMS);
        expect(state.isOpen('settings')).toBe(false);

        state.setOpen('settings', true);
        expect(state.isOpen('settings')).toBe(true);
    });
});
