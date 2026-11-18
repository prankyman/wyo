import { describe, expect, it } from 'vitest';
import { NAV_ITEM_DATA } from '$lib/nav/nav-items-data';
import ru from '../../../messages/ru.json';
import { MENU_PLACE_IDS, menuPlaceText, resolveMenuPlaces, type IMenuSource, type TLabelOf } from './menu-places';

const labelOf: TLabelOf = (key) => (ru as Record<string, string>)[key];

/** A made-up menu with a group: the real menu holds none yet, so the rule for an item inside a group is tested on this one. */
const GROUPED: IMenuSource = {
    items: [
        { labelKey: 'x.top', href: '/', testId: 'nav-top' },
        { labelKey: 'x.inside', href: '/inside', testId: 'nav-inside', section: 'g' }
    ],
    sectionLabelKeys: { g: 'x.group' }
};
const GROUPED_LABELS: TLabelOf = (key) =>
    ({
        'x.top': 'Верх',
        'x.inside': 'Внутри',
        'x.group': 'Группа',
        'menu.place.top': ru['menu.place.top'],
        'menu.place.inGroup': ru['menu.place.inGroup']
    })[key];

describe('menuPlaceText', () => {
    it('writes every item of the real menu in quotes from its label', () => {
        expect(MENU_PLACE_IDS).toEqual(NAV_ITEM_DATA.map((item) => item.testId));
        for (const item of NAV_ITEM_DATA) {
            expect(menuPlaceText(item.testId, labelOf)).toBe(`«${labelOf(item.labelKey)}»`);
        }
        expect(menuPlaceText('nav-settings', labelOf)).toBe('«Настройки»');
    });

    it('writes an item above the groups alone and an item of a group after the group’s name', () => {
        expect(menuPlaceText('nav-top', GROUPED_LABELS, GROUPED)).toBe('«Верх»');
        expect(menuPlaceText('nav-inside', GROUPED_LABELS, GROUPED)).toBe('«Группа → Внутри»');
    });

    it('throws on an id no menu holds and on a label that has no text', () => {
        expect(() => menuPlaceText('nav-nothing', labelOf)).toThrow('No menu item has the id nav-nothing');
        expect(() => menuPlaceText('nav-overview', () => undefined)).toThrow('has no text');
    });
});

describe('resolveMenuPlaces', () => {
    it('writes out every token of a text and leaves the rest as it is', () => {
        expect(resolveMenuPlaces('Откройте {menu:nav-settings}, затем {menu:nav-overview}.', labelOf)).toBe(
            'Откройте «Настройки», затем «Обзор».'
        );
    });

    it('returns a text without a token unchanged, other braces and an empty token alike, and a value that is not a text as it is', () => {
        expect(resolveMenuPlaces('Строка {count} без меню', labelOf)).toBe('Строка {count} без меню');
        expect(resolveMenuPlaces('{menu:}', labelOf)).toBe('{menu:}');
        expect(resolveMenuPlaces(undefined as unknown as string, labelOf)).toBeUndefined();
    });
});
