import GearSixIcon from 'phosphor-svelte/lib/GearSixIcon';
import GaugeIcon from 'phosphor-svelte/lib/GaugeIcon';
import { NAV_ITEM_DATA, NAV_SECTIONS } from './nav-items-data';
import type { INavItem, TIconComponent, TNavSection } from './types';

export type { INavCounter, INavItem, INavItemData, TIconComponent, TNavSection } from './types';
export { NAV_ITEM_DATA, NAV_SECTIONS, NAV_SECTION_LABEL_KEYS } from './nav-items-data';

/** What fits across the bottom of a phone next to «Ещё». */
export const PHONE_TAB_LIMIT = 5;

/** Each item's icon, by its `data-test` id: a missing or unknown id fails the type check. */
const NAV_ICONS: Record<(typeof NAV_ITEM_DATA)[number]['testId'], TIconComponent> = {
    'nav-overview': GaugeIcon,
    'nav-settings': GearSixIcon
};

/** The menu as drawn: `NAV_ITEM_DATA` with the icons. */
export const NAV_ITEMS: readonly INavItem[] = NAV_ITEM_DATA.map((item) => ({ ...item, icon: NAV_ICONS[item.testId] }));

/**
 * The phone's split of the items: the strip takes the five with the lowest `phoneTab` ranks, in rank order; every other item goes to
 * the «Ещё» sheet.
 */
export function phoneStrip(items: readonly INavItem[]): { strip: INavItem[]; secondary: INavItem[] } {
    const strip = items
        .filter((item) => item.phoneTab !== undefined)
        .sort((a, b) => (a.phoneTab ?? 0) - (b.phoneTab ?? 0))
        .slice(0, PHONE_TAB_LIMIT);
    return { strip, secondary: items.filter((item) => !strip.includes(item)) };
}

/** One group of the menu as drawn: its items in the list's order. */
export interface INavGroup {
    section: TNavSection;
    items: INavItem[];
}

/**
 * The items folded into groups: those of no group on top, then every group of `sections` in its order. A group with no item here
 * is left out, so a menu never shows an empty heading.
 */
export function navGroups(
    items: readonly INavItem[],
    sections: readonly TNavSection[] = NAV_SECTIONS
): { top: INavItem[]; groups: INavGroup[] } {
    return {
        top: items.filter((item) => item.section === undefined),
        groups: sections
            .map((section) => ({ section, items: items.filter((item) => item.section === section) }))
            .filter((group) => group.items.length > 0)
    };
}

/** The group of the item the open page belongs to; null on a page of no group or one outside the menu. */
export function sectionOfPath(items: readonly INavItem[], currentPath: string): TNavSection | null {
    return items.find((item) => isNavItemActive(item, currentPath))?.section ?? null;
}

/**
 * The groups open on a new visit: only the open page's group. On the home page the first group drawn is open; a page outside the
 * menu opens none.
 */
export function initialOpenSections(
    drawn: readonly TNavSection[],
    current: TNavSection | null,
    home: boolean
): TNavSection[] {
    if (current && drawn.includes(current)) return [current];
    if (!home) return [];
    return drawn.length > 0 ? [drawn[0]] : [];
}

/** Whether the page is the item's own page or one under it (`/` and an `exact` item: their own page only). */
function ownsPath(item: INavItem, currentPath: string): boolean {
    if (item.href === '/' || item.exact) return currentPath === item.href;
    return currentPath === item.href || currentPath.startsWith(`${item.href}/`);
}

/**
 * Whether the item is the page the person is on. `/` matches only itself: it is a prefix of every address, so a prefix test would
 * light «Обзор» on every page. Given the menu's `items`, a page that lies under two items (`/stock` and `/stock/counts`) lights only
 * the item with the longest address, so one page lights exactly one item.
 */
export function isNavItemActive(item: INavItem, currentPath: string, items: readonly INavItem[] = []): boolean {
    if (item.alsoActive?.some((path) => currentPath === path || currentPath.startsWith(`${path}/`))) return true;
    if (!ownsPath(item, currentPath)) return false;
    return !items.some(
        (other) => other !== item && other.href.length > item.href.length && ownsPath(other, currentPath)
    );
}
