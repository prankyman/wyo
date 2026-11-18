import { describe, expect, it } from 'vitest';
import type { INotificationLine } from '$lib/notifications/types';
import {
    announcement,
    attentionKeys,
    bellNumber,
    bellTone,
    panelKeys,
    shouldPulse,
    URGENCY_STYLE
} from './panel-rules';

const line = (key: string, urgency: INotificationLine['urgency']): INotificationLine => ({
    key,
    kind: key,
    urgency,
    text: `text ${key}`,
    href: '/settings'
});

describe('the bell’s look', () => {
    it.each([
        ['nothing', [], 'quiet'],
        ['information only', [line('a', 'info')], 'attention'],
        ['an attention line', [line('a', 'attention')], 'attention'],
        ['an urgent line', [line('a', 'info'), line('b', 'urgent')], 'urgent']
    ] as const)('%s → %s', (_case, lines, tone) => {
        expect(bellTone(lines)).toBe(tone);
    });

    it.each([
        [0, null],
        [-1, null],
        [1, '1'],
        [99, '99'],
        [100, '99+']
    ])('writes %s lines as %s', (count, number) => {
        expect(bellNumber(count)).toBe(number);
    });

    it('colours the three urgencies from the theme and gives each its own border', () => {
        expect(Object.keys(URGENCY_STYLE)).toEqual(['urgent', 'attention', 'info']);
        expect(URGENCY_STYLE.urgent).toEqual({ border: 'border-stop-600', background: 'bg-stop-50' });
    });
});

describe('the pulse', () => {
    it('compares the keys of the urgent lines: a new one pulses, a seen one does not', () => {
        const lines = [line('a', 'urgent'), line('b', 'attention')];
        expect(attentionKeys(lines)).toEqual(['line:a']);
        expect(shouldPulse(attentionKeys(lines), new Set())).toBe(true);
        expect(shouldPulse(attentionKeys(lines), new Set(['line:a']))).toBe(false);
        expect(shouldPulse([], new Set())).toBe(false);
    });
});

describe('announcement', () => {
    it('says nothing on the first answer, says the text of a new line, and nothing when nothing is new', () => {
        const lines = [line('a', 'info'), line('b', 'info')];
        expect(announcement(null, lines)).toBeNull();
        expect(announcement(new Set(['line:a']), lines)).toBe('text b');
        expect(announcement(panelKeys(lines), lines)).toBeNull();
    });
});
