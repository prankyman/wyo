import type GaugeIcon from 'phosphor-svelte/lib/GaugeIcon';

/**
 * A phosphor-svelte icon component. Derived from a real icon rather than hand-written, because phosphor's own prop type carries
 * an index signature for `data-*` attributes that a hand-written interface does not — and a near-miss here rejects every icon.
 */
export type TIconComponent = typeof GaugeIcon;

/**
 * The name of a menu group. The menu is folded into groups when a plan adds enough items to need them: a group is a name here, its
 * label key in `NAV_SECTION_LABEL_KEYS` and its place in `NAV_SECTIONS`. The list holds none today, so no group is drawn.
 */
export type TNavSection = string;

/** One destination of the menu, without its icon (`nav-items-data.ts`). */
export interface INavItemData {
    /** The key of the label in `messages/`. */
    labelKey: string;
    /** The address, without a language prefix; the menu writes the prefix. */
    href: string;
    /** The item's `data-test`, and the id a menu token names (`{menu:nav-settings}`, `menu-places.ts`). */
    testId: string;
    /** The group the item is drawn in; none for an item above the groups. */
    section?: TNavSection;
    /**
     * Offered for the phone strip, with its rank there. A phone's strip holds the five items of the lowest ranks
     * (`phoneStrip`) and every other item is reached from the «Ещё» sheet.
     */
    phoneTab?: number;
    /** The item is the open page on its own address only, never on a page under it. */
    exact?: true;
    /** Other addresses on which the item is the open page too. */
    alsoActive?: readonly string[];
}

/** One destination of the menu as drawn: the data and its icon. */
export interface INavItem extends INavItemData {
    icon: TIconComponent;
}

/** A count drawn beside an item (`CounterPill`) with the sentence that says what it counts. A count of 0 is not drawn. */
export interface INavCounter {
    count: number;
    sentence: string;
}
