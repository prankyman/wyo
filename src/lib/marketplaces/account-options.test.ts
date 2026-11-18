import { describe, expect, it } from 'vitest';
import { accountOf } from '$test/support/marketplaces';
import { accountNameOf, accountOptions } from './account-options';

const ACCOUNTS = [accountOf(), accountOf({ id: 8, name: 'Второй магазин' })];

describe('the accounts of a list’s filter', () => {
    it('offers every account by its own name, its id as the value', () => {
        expect(accountOptions(ACCOUNTS)).toEqual([
            { value: '7', label: 'Магазин одежды' },
            { value: '8', label: 'Второй магазин' }
        ]);
    });

    it('names an account by its id, and by the id itself when the account is gone', () => {
        expect(accountNameOf(ACCOUNTS, '8')).toBe('Второй магазин');
        expect(accountNameOf(ACCOUNTS, '99')).toBe('99');
    });
});
