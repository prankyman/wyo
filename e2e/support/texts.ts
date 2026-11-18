import { readFileSync } from 'node:fs';
import path from 'node:path';

/**
 * The words of the wyo interface, read from the file the interface itself is built from (`messages/ru.json`), so a spec asserts
 * the text a person reads without copying it. The file is flat: one key, one text, `{name}` where a value goes.
 */
const MESSAGES: Record<string, string> = JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../../messages/ru.json'), 'utf8')
) as Record<string, string>;

function messageOf(key: string): string {
    const text = MESSAGES[key];
    if (text === undefined) throw new Error(`messages/ru.json has no key ${key}`);
    return text;
}

/**
 * A menu place written out the way the interface writes it (`src/lib/i18n/menu-places.ts`): `{menu:nav-settings}` is the menu item
 * `nav.settings` set in the quotes of `menu.place.top`. Only items above the menu's groups exist today.
 */
function writeMenuPlaces(text: string): string {
    return text.replace(/\{menu:nav-([A-Za-z0-9-]+)\}/g, (_token, item: string) =>
        messageOf('menu.place.top').replace('{item}', messageOf(`nav.${item}`))
    );
}

/** The text of `key` with `{name}` filled from `params` and its menu places written out. Throws for a key the file lacks. */
export function ruText(key: string, params: Record<string, string | number> = {}): string {
    const filled = Object.entries(params).reduce(
        (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
        messageOf(key)
    );
    return writeMenuPlaces(filled);
}
