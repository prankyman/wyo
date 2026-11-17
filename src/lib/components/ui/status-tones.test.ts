import { describe, expect, it } from 'vitest';
import { TONE_CLASS, TONE_TEXT_CLASS, toneClass, type TTone } from './status-tones';

/** The six looks of a status: pinned, so a change of a shade is a decision, not an accident. */
describe('the status tones', () => {
    it.each<[TTone, string, string]>([
        ['ok', 'bg-ok-50 text-ok-800', 'text-ok-800'],
        ['wait', 'bg-wait-50 text-wait-800', 'text-wait-800'],
        ['warn', 'bg-warn-50 text-warn-800', 'text-warn-800'],
        ['stop', 'bg-stop-50 text-stop-800', 'text-stop-800'],
        ['info', 'bg-brand-50 text-brand-800', 'text-brand-800'],
        ['off', 'bg-ink-100 text-ink-600', 'text-ink-600']
    ])('draws %s as %s, and as words in %s', (tone, badge, words) => {
        expect(toneClass(tone)).toBe(badge);
        expect(TONE_CLASS[tone]).toBe(badge);
        expect(TONE_TEXT_CLASS[tone]).toBe(words);
    });
});
