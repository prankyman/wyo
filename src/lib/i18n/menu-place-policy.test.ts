import { describe, expect, it } from 'vitest';
import { NAV_ITEM_DATA, NAV_SECTION_LABEL_KEYS } from '$lib/nav/nav-items-data';
import { DEFAULT_LOCALE, messages } from './messages';
import { MENU_PLACE_IDS, MENU_TOKEN } from './menu-places';

/**
 * A menu place is never written by hand (constitution, help-text rule 9). A text writes `{menu:<id>}` and `t()` writes the place out
 * from the menu itself (`menu-places.ts`), so a text follows an item that moves to another group or is renamed. This test fails:
 *
 * - a token whose id no menu holds;
 * - a text that writes a menu group's label in quotes followed by «→» («Группа» → …, «Группа → …»);
 * - a Russian text that writes a menu item's or group's label in quotes after «в меню» or «в разделе».
 */

const escape = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

/** The quoted group label followed by an arrow; null while the menu has no group. */
function groupArrowPattern(groupLabels: readonly string[]): RegExp | null {
    if (groupLabels.length === 0) return null;
    return new RegExp(`[«“「](?:${groupLabels.map(escape).join('|')})(?:[»”」]\\s*→|\\s*→)`);
}

/** The quoted menu label after «в меню» or «в разделе». */
function inMenuPattern(labels: readonly string[]): RegExp {
    return new RegExp(`в (?:меню|разделе) «(?:${labels.map(escape).join('|')})»`);
}

const russian = messages[DEFAULT_LOCALE];
const GROUP_ARROW = groupArrowPattern(Object.values(NAV_SECTION_LABEL_KEYS).map((key) => russian[key]));
const IN_MENU = inMenuPattern([
    ...NAV_ITEM_DATA.map((item) => russian[item.labelKey]),
    ...Object.values(NAV_SECTION_LABEL_KEYS).map((key) => russian[key])
]);

/** Every text of every language that breaks the rule, as `locale key: reason`. */
function menuPlaceFailures(): string[] {
    const failures: string[] = [];
    for (const [locale, bundle] of Object.entries(messages)) {
        for (const [key, text] of Object.entries(bundle)) {
            for (const match of text.matchAll(MENU_TOKEN)) {
                if (!MENU_PLACE_IDS.includes(match[1])) failures.push(`${locale} ${key}: no menu item ${match[1]}`);
            }
            if (GROUP_ARROW?.test(text)) failures.push(`${locale} ${key}: a menu group written by hand before «→»`);
            if (locale === DEFAULT_LOCALE && IN_MENU.test(text)) {
                failures.push(`${locale} ${key}: a menu label after «в меню»/«в разделе»`);
            }
        }
    }
    return failures;
}

describe('menu places are written from the menu (constitution, help-text rule 9)', () => {
    it('fails a place written by hand and passes a token', () => {
        const arrow = groupArrowPattern(['Группа']);
        expect(arrow?.test('откройте «Группа» → «Пункт»')).toBe(true);
        expect(arrow?.test('откройте «Группа → Пункт»')).toBe(true);
        expect(arrow?.test('откройте {menu:nav-settings}')).toBe(false);
        expect(groupArrowPattern([])).toBeNull();
        expect(inMenuPattern(['Настройки']).test('в разделе «Настройки»')).toBe(true);
        expect(inMenuPattern(['Настройки']).test('в разделе «Ячейки»')).toBe(false);
        expect(IN_MENU.test('Откройте в меню «Настройки»')).toBe(true);
    });

    it('finds no text that breaks the rule', () => {
        expect(menuPlaceFailures()).toEqual([]);
    });
});
