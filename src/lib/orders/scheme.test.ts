import { describe, expect, it } from 'vitest';
import { sellerPacks } from './scheme';

describe('sellerPacks', () => {
    it.each([
        ['fbs', true],
        ['express', true],
        ['fbo', false],
        ['fby', false]
    ] as const)('says whether the seller packs an order of the scheme %s: %s', (scheme, packs) => {
        expect(sellerPacks(scheme)).toBe(packs);
    });
});
