import { describe, expect, it } from 'vitest';
import { splitLocale, withLocale } from './locale-path';

describe('splitLocale', () => {
    const locales = ['ru', 'xx'];

    it.each([
        ['/xx/showcase', 'xx', '/showcase'],
        ['/xx', 'xx', '/'],
        ['/xx/', 'xx', '/'],
        ['/showcase', 'ru', '/showcase'],
        ['/ru/showcase', 'ru', '/ru/showcase'],
        ['/yy/showcase', 'ru', '/yy/showcase'],
        ['/', 'ru', '/']
    ])('reads %s as language %s, page %s', (pathname, locale, path) => {
        expect(splitLocale(pathname, locales, 'ru')).toEqual({ locale, path });
    });
});

describe('withLocale', () => {
    it('leaves the default language unprefixed and prefixes the others', () => {
        expect(withLocale('/showcase', 'ru', 'ru')).toBe('/showcase');
        expect(withLocale('/showcase', 'xx', 'ru')).toBe('/xx/showcase');
        expect(withLocale('/', 'xx', 'ru')).toBe('/xx');
    });
});
