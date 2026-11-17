import { describe, expect, it } from 'vitest';
import { cn } from './cn';

describe('cn', () => {
    it('joins class names and skips what is not a name', () => {
        expect(cn('a', false, undefined, null, 'b', { c: true, d: false }, ['e'])).toBe('a b c e');
    });

    it('lets the last of two conflicting Tailwind classes win', () => {
        expect(cn('p-2 text-ink-800', 'p-4')).toBe('text-ink-800 p-4');
        expect(cn('bg-brand-600', 'bg-ok-50')).toBe('bg-ok-50');
    });
});
