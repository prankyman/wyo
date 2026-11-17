import { afterEach, describe, expect, it } from 'vitest';
import { messages } from '$lib/i18n/messages';
import { reroute } from './hooks';

/**
 * The language prefix of an address. A second language exists in these tests only: it is put into the table of texts for the
 * length of one test (a file `messages/xx.json` does the same in the built program).
 */
const SECOND = 'xx';
const route = (pathname: string) => reroute({ url: new URL(`http://localhost${pathname}`), fetch });

afterEach(() => {
    delete messages[SECOND];
});

describe('reroute', () => {
    it('changes no address while Russian is the only language', () => {
        for (const pathname of ['/', '/showcase', `/${SECOND}/showcase`, '/ru/showcase'])
            expect(route(pathname), pathname).toBeUndefined();
    });

    it('maps an address of a language that exists to the page without the prefix', () => {
        messages[SECOND] = {};

        expect(route(`/${SECOND}/showcase`)).toBe('/showcase');
        expect(route(`/${SECOND}/`)).toBe('/');
        expect(route(`/${SECOND}`)).toBe('/');
        expect(route(`/${SECOND}/a/b/c`)).toBe('/a/b/c');
    });

    it('leaves an unprefixed address, the default language and an unknown prefix alone', () => {
        messages[SECOND] = {};

        expect(route('/showcase')).toBeUndefined();
        expect(route('/ru/showcase')).toBeUndefined();
        expect(route('/yy/showcase')).toBeUndefined();
        expect(route(`/showcase/${SECOND}`)).toBeUndefined();
    });
});
