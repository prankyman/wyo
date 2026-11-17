import { afterEach, describe, expect, it, vi } from 'vitest';
import { copyToClipboard } from './clipboard';

/** The one copy to the clipboard. */
afterEach(() => {
    vi.unstubAllGlobals();
});

describe('copyToClipboard', () => {
    it('puts the text on the clipboard and says it did', async () => {
        const writeText = vi.fn(async () => undefined);
        vi.stubGlobal('navigator', { clipboard: { writeText } });

        await expect(copyToClipboard('wyo --token abc')).resolves.toBe(true);
        expect(writeText).toHaveBeenCalledWith('wyo --token abc');
    });

    it.each([
        ['the browser refuses', { clipboard: { writeText: vi.fn(async () => Promise.reject(new Error('denied'))) } }],
        ['the browser has no clipboard', {}]
    ])('says it did not when %s', async (_case, navigator) => {
        vi.stubGlobal('navigator', navigator);

        await expect(copyToClipboard('text')).resolves.toBe(false);
    });
});
