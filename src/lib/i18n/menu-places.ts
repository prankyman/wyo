import { NAV_ITEM_DATA, NAV_SECTION_LABEL_KEYS } from '$lib/nav/nav-items-data';
import type { INavItemData, TNavSection } from '$lib/nav/types';

/**
 * Menu places written from the menu itself (constitution, help-text rule 9). A text that tells the reader where something is in
 * the menu writes a token instead of the place: `{menu:<testId>}` for an item of the menu (`src/lib/nav/nav-items-data.ts`, for
 * example `{menu:nav-settings}`). The text system writes the token out from the same list the menu is drawn from, so a text follows
 * an item that moves to another group or is renamed. How a place is written — the quotes and the arrow — is two texts of
 * `messages/` (`menu.place.top`, `menu.place.inGroup`), so a second language writes it its own way with no change in code.
 */

/** A menu token in a text; the first group is the id: `nav-settings`. */
export const MENU_TOKEN = /\{menu:([A-Za-z0-9-]+)\}/g;

/** The text of a message key in the reader's language, or undefined when there is none. */
export type TLabelOf = (key: string) => string | undefined;

/** What a place is written from: the menu's items and the label key of each group. */
export interface IMenuSource {
    items: readonly INavItemData[];
    sectionLabelKeys: Readonly<Record<TNavSection, string>>;
}

const MENU: IMenuSource = { items: NAV_ITEM_DATA, sectionLabelKeys: NAV_SECTION_LABEL_KEYS };

/** Every id a token may name: the menu items' `testId`s. */
export const MENU_PLACE_IDS: readonly string[] = NAV_ITEM_DATA.map((item) => item.testId);

function label(labelOf: TLabelOf, key: string): string {
    const text = labelOf(key);
    if (text === undefined) throw new Error(`The menu label ${key} has no text`);
    return text;
}

/**
 * The menu place `id` names: «Группа → Пункт» for an item in a group, «Пункт» for one above the groups. `labelOf` reads the
 * texts in the reader's language. An id that no menu holds throws: the text naming it is wrong (`menu-place-policy.test.ts`
 * finds it before a reader does).
 */
export function menuPlaceText(id: string, labelOf: TLabelOf, menu: IMenuSource = MENU): string {
    const item = menu.items.find((entry) => entry.testId === id);
    if (item === undefined) throw new Error(`No menu item has the id ${id}`);
    const itemLabel = label(labelOf, item.labelKey);
    if (item.section === undefined) return label(labelOf, 'menu.place.top').replace('{item}', itemLabel);
    const group = label(labelOf, menu.sectionLabelKeys[item.section]);
    return label(labelOf, 'menu.place.inGroup').replace('{group}', group).replace('{item}', itemLabel);
}

/**
 * `text` with every menu token written out; a text without a token is returned as it is, and so is a value that is not a text
 * at all (`t()` of a key that is undefined returns it unchanged).
 */
export function resolveMenuPlaces(text: string, labelOf: TLabelOf, menu: IMenuSource = MENU): string {
    if (typeof text !== 'string' || !text.includes('{menu:')) return text;
    return text.replace(MENU_TOKEN, (_token, id: string) => menuPlaceText(id, labelOf, menu));
}
