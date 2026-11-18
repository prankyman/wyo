import { describe, expect, it } from 'vitest';
import { MARKETPLACE_CODE, SECOND_MARKETPLACE_CODE } from '$test/support/marketplaces';
import { accountRefusalLookup, accountTextKey } from './refusals';

const OWN = (marketplace: string, kind: string, code: string) => `marketplace.${marketplace}.${kind}.${code}`;

/** The one lookup of the text of an account's refusal or note: the marketplace's own text first, then the shared one. */
describe('the text of an account’s refusal or note', () => {
    it.each([
        ['a refusal only its marketplace words', 'refusal', 'key_type_not_accepted', MARKETPLACE_CODE, 'own'],
        ['a refusal both words, its marketplace’s first', 'refusal', 'key_refused', MARKETPLACE_CODE, 'own'],
        [
            'a refusal the marketplace does not word, the shared one',
            'refusal',
            'key_unreadable',
            MARKETPLACE_CODE,
            'shared'
        ],
        ['a refusal of the second marketplace', 'refusal', 'key_deactivated', SECOND_MARKETPLACE_CODE, 'own'],
        [
            'a shared refusal for the second marketplace',
            'refusal',
            'profile_mismatch',
            SECOND_MARKETPLACE_CODE,
            'shared'
        ],
        ['the note of a test key', 'note', 'test', MARKETPLACE_CODE, 'own'],
        ['the note of a missing category', 'note', 'missing.statistics', MARKETPLACE_CODE, 'own'],
        [
            'the note of a missing category of the second marketplace',
            'note',
            'missing.posting_fbo',
            SECOND_MARKETPLACE_CODE,
            'own'
        ],
        ['the shared note of a category nobody worded', 'note', 'missingCategory', SECOND_MARKETPLACE_CODE, 'shared']
    ] as const)('finds %s', (_case, kind, code, marketplace, where) => {
        expect(accountTextKey(kind, code, marketplace)).toBe(
            where === 'own' ? OWN(marketplace, kind, code) : `accounts.${kind}.${code}`
        );
    });

    it('finds the shared text only when the marketplace is not named or is not known', () => {
        expect(accountTextKey('refusal', 'key_refused')).toBe('accounts.refusal.key_refused');
        expect(accountTextKey('refusal', 'key_refused', 'later-one')).toBe('accounts.refusal.key_refused');
        expect(accountTextKey('refusal', 'key_type_not_accepted')).toBeNull();
    });

    it.each([
        ['a note the marketplace has no text for', 'note', 'test', SECOND_MARKETPLACE_CODE],
        ['a category nobody has a text for', 'note', 'missing.content', MARKETPLACE_CODE],
        ['a refusal nobody has a text for', 'refusal', 'never_heard_of_it', MARKETPLACE_CODE]
    ] as const)('finds nothing for %s', (_case, kind, code, marketplace) => {
        expect(accountTextKey(kind, code, marketplace)).toBeNull();
    });

    it('finds a note written in counted forms by its base key', () => {
        expect(accountTextKey('note', 'sellerDelivers', SECOND_MARKETPLACE_CODE)).toBe(
            OWN(SECOND_MARKETPLACE_CODE, 'note', 'sellerDelivers')
        );
    });

    it('gives the forms and the buttons a lookup for one marketplace, which answers the same keys', () => {
        const lookup = accountRefusalLookup(SECOND_MARKETPLACE_CODE);

        expect(lookup('key_deactivated')).toBe(OWN(SECOND_MARKETPLACE_CODE, 'refusal', 'key_deactivated'));
        expect(lookup('too_soon')).toBe('accounts.refusal.too_soon');
        expect(lookup('never_heard_of_it')).toBeNull();
    });
});
