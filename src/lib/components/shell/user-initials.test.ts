import { describe, expect, it } from 'vitest';
import { loginInitials } from './user-initials';

describe('loginInitials', () => {
    it.each([
        ['owner', 'O'],
        ['анна', 'А'],
        ['  seller.one', 'S'],
        ['', '?']
    ])('writes «%s» as %s', (login, initials) => {
        expect(loginInitials(login)).toBe(initials);
    });
});
