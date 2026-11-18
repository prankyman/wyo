import type { INavItemData, TNavSection } from './types';

/**
 * The menu's data without its icons: the label, the address, the `data-test` id and the rules. It is apart from the icons so
 * that the translation function (`menuPlaceText`, `src/lib/i18n/menu-places.ts`) can read the menu without loading an icon;
 * `nav-items.ts` adds the icons and is what the menu draws from.
 *
 * This is the ONE list of the menu, for both the computer's left column and the phone's bottom strip. A later plan adds its
 * item here and nowhere else; the menu, the phone strip and every text that tells where something is in the menu follow.
 */
export const NAV_ITEM_DATA = [
    { labelKey: 'nav.overview', href: '/', testId: 'nav-overview', phoneTab: 1 },
    { labelKey: 'nav.orders', href: '/orders', testId: 'nav-orders', phoneTab: 2 },
    { labelKey: 'nav.handovers', href: '/handovers', testId: 'nav-handovers' },
    { labelKey: 'nav.marketplaces', href: '/marketplaces', testId: 'nav-marketplaces', phoneTab: 3 },
    { labelKey: 'nav.externalApi', href: '/external-api', testId: 'nav-external-api' },
    { labelKey: 'nav.journal', href: '/journal', testId: 'nav-journal', phoneTab: 4 },
    { labelKey: 'nav.settings', href: '/settings', testId: 'nav-settings', phoneTab: 5 }
] as const satisfies readonly INavItemData[];

/** The menu's groups in the order they are drawn. None is needed for seven items. */
export const NAV_SECTIONS: readonly TNavSection[] = [];

/** The label of each group, written out so the check for unused message keys finds every one. */
export const NAV_SECTION_LABEL_KEYS: Readonly<Record<TNavSection, string>> = {};
