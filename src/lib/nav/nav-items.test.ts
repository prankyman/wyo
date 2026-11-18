import { describe, expect, it } from 'vitest';
import {
    initialOpenSections,
    isNavItemActive,
    navGroups,
    NAV_ITEMS,
    NAV_ITEM_DATA,
    PHONE_TAB_LIMIT,
    phoneStrip,
    sectionOfPath
} from './nav-items';
import type { INavItem, TNavSection } from './types';

/** An item of a made-up menu with groups: the real list holds none yet, so the group rules are tested on this one. */
function item(testId: string, href: string, extra: Partial<INavItem> = {}): INavItem {
    return { labelKey: `x.${testId}`, href, testId, icon: NAV_ITEMS[0].icon, ...extra };
}

const GROUPED: INavItem[] = [
    item('nav-overview', '/'),
    item('nav-orders', '/orders', { section: 'orders' as TNavSection }),
    item('nav-journal', '/orders/journal', { section: 'orders' as TNavSection }),
    item('nav-settings', '/settings', { section: 'settings' as TNavSection })
];

describe('NAV_ITEMS, the one list the menu is drawn from', () => {
    it('holds «Обзор», «Заказы», «Передачи», «Маркетплейсы», «Внешний API», «Журнал запросов» and «Настройки» in this order, each with an address, a label key, a test id and an icon', () => {
        expect(NAV_ITEMS.map((entry) => [entry.testId, entry.href, entry.labelKey])).toEqual([
            ['nav-overview', '/', 'nav.overview'],
            ['nav-orders', '/orders', 'nav.orders'],
            ['nav-handovers', '/handovers', 'nav.handovers'],
            ['nav-marketplaces', '/marketplaces', 'nav.marketplaces'],
            ['nav-external-api', '/external-api', 'nav.externalApi'],
            ['nav-journal', '/journal', 'nav.journal'],
            ['nav-settings', '/settings', 'nav.settings']
        ]);
        expect(NAV_ITEMS.every((entry) => typeof entry.icon === 'function' || typeof entry.icon === 'object')).toBe(
            true
        );
    });

    it('gives every entry a distinct address, test id and label key, and offers the first five pages to the phone strip while «Передачи» and «Внешний API» wait in the «Ещё» sheet', () => {
        for (const field of ['href', 'testId', 'labelKey'] as const) {
            expect(new Set(NAV_ITEM_DATA.map((entry) => entry[field])).size).toBe(NAV_ITEM_DATA.length);
        }
        expect(NAV_ITEMS.map((entry) => entry.phoneTab)).toEqual([1, 2, undefined, 3, undefined, 4, 5]);
        const { strip, secondary } = phoneStrip(NAV_ITEMS);
        expect(strip.map((entry) => entry.testId)).toEqual([
            'nav-overview',
            'nav-orders',
            'nav-marketplaces',
            'nav-journal',
            'nav-settings'
        ]);
        expect(secondary.map((entry) => entry.testId)).toEqual(['nav-handovers', 'nav-external-api']);
    });
});

describe('isNavItemActive', () => {
    it.each([
        ['/', '/', true],
        ['/', '/settings', false],
        ['/settings', '/settings', true],
        ['/settings', '/settings/keys', true],
        ['/settings', '/settingsx', false]
    ])('item %s on page %s is %s', (href, path, active) => {
        expect(isNavItemActive(item('nav-x', href), path)).toBe(active);
    });

    it('matches an `exact` item on its own path only, and an item on its `alsoActive` paths and the pages under them', () => {
        expect(isNavItemActive(item('nav-x', '/months', { exact: true }), '/months/2026')).toBe(false);
        expect(isNavItemActive(item('nav-x', '/months', { exact: true }), '/months')).toBe(true);
        const also = item('nav-x', '/help', { alsoActive: ['/faq'] });
        expect(isNavItemActive(also, '/faq')).toBe(true);
        expect(isNavItemActive(also, '/faq/one')).toBe(true);
        expect(isNavItemActive(also, '/other')).toBe(false);
    });

    it('lights only the item with the longest matching address when one page lies under two items', () => {
        const items = [item('nav-stock', '/stock'), item('nav-counts', '/stock/counts')];
        expect(items.map((entry) => isNavItemActive(entry, '/stock/counts', items))).toEqual([false, true]);
        expect(items.map((entry) => isNavItemActive(entry, '/stock', items))).toEqual([true, false]);
    });
});

describe('phoneStrip', () => {
    it('puts the items with the lowest ranks on the strip, in rank order, and the rest in the sheet', () => {
        const ranked = Array.from({ length: PHONE_TAB_LIMIT + 2 }, (_, index) =>
            item(`nav-${index}`, `/p${index}`, { phoneTab: PHONE_TAB_LIMIT + 2 - index })
        );
        ranked.push(item('nav-unranked', '/u'));

        const { strip, secondary } = phoneStrip(ranked);

        expect(strip.map((entry) => entry.testId)).toEqual(['nav-6', 'nav-5', 'nav-4', 'nav-3', 'nav-2']);
        expect(secondary.map((entry) => entry.testId)).toEqual(['nav-0', 'nav-1', 'nav-unranked']);
    });
});

describe('the menu groups', () => {
    it('draws the items of no group on top and only the groups that hold an item, in the list’s order', () => {
        const { top, groups } = navGroups(GROUPED, ['orders', 'settings', 'empty']);

        expect(top.map((entry) => entry.testId)).toEqual(['nav-overview']);
        expect(groups.map((group) => [group.section, group.items.map((entry) => entry.testId)])).toEqual([
            ['orders', ['nav-orders', 'nav-journal']],
            ['settings', ['nav-settings']]
        ]);
    });

    it('names the group of the page the person is on, and none for a page of no group or outside the menu', () => {
        expect(sectionOfPath(GROUPED, '/orders/journal')).toBe('orders');
        expect(sectionOfPath(GROUPED, '/')).toBeNull();
        expect(sectionOfPath(GROUPED, '/nowhere')).toBeNull();
    });

    it.each([
        ['the page’s own group alone', ['orders', 'settings'], 'settings', false, ['settings']],
        ['the first group on the home page', ['orders', 'settings'], null, true, ['orders']],
        ['no group on a page outside the menu', ['orders', 'settings'], null, false, []],
        ['nothing when no group is drawn', [], null, true, []],
        ['the first drawn group when «orders» is not drawn', ['settings'], null, true, ['settings']]
    ] as const)('opens %s on a new visit', (_name, drawn, current, home, opened) => {
        expect(initialOpenSections(drawn, current, home)).toEqual(opened);
    });
});
