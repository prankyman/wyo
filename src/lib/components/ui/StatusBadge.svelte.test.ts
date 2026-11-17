import { render } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import StatusBadge from './StatusBadge.svelte';

describe('StatusBadge', () => {
    it('renders the label text', () => {
        const { container } = render(StatusBadge, { label: 'Confirmed', colorClass: 'bg-wait-100 text-wait-800' });
        expect(container.querySelector('span')?.textContent).toBe('Confirmed');
    });

    it('applies the passed colorClass', () => {
        const { container } = render(StatusBadge, { label: 'Pending', colorClass: 'bg-warn-100 text-warn-800' });
        const el = container.querySelector('span');
        expect(el?.className).toContain('bg-warn-100');
        expect(el?.className).toContain('text-warn-800');
    });

    it('does not hardcode any status colors of its own', () => {
        const { container } = render(StatusBadge, { label: 'Custom', colorClass: 'bg-brand-100 text-brand-900' });
        const el = container.querySelector('span');
        expect(el?.className).toContain('bg-brand-100');
        expect(el?.className).toContain('text-brand-900');
        expect(el?.className).not.toContain('bg-warn-100');
        expect(el?.className).not.toContain('bg-wait-100');
    });

    it('uses the small size classes by default', () => {
        const { container } = render(StatusBadge, { label: 'Pending', colorClass: 'bg-warn-100 text-warn-800' });
        const el = container.querySelector('span');
        expect(el?.className).toContain('px-2');
        expect(el?.className).toContain('py-0.5');
        expect(el?.className).toContain('text-[10px]');
    });

    it('uses the medium size classes when size="md"', () => {
        const { container } = render(StatusBadge, {
            label: 'Pending',
            colorClass: 'bg-warn-100 text-warn-800',
            size: 'md'
        });
        const el = container.querySelector('span');
        expect(el?.className).toContain('px-3');
        expect(el?.className).toContain('py-1');
        expect(el?.className).toContain('text-xs');
    });

    it('applies dataTest prop as data-test attribute when provided', () => {
        const { container } = render(StatusBadge, {
            label: 'Pending',
            colorClass: 'bg-warn-100 text-warn-800',
            dataTest: 'order-status'
        });
        expect(container.querySelector('[data-test="order-status"]')).not.toBeNull();
    });

    it('omits data-test attribute when dataTest is not provided', () => {
        const { container } = render(StatusBadge, { label: 'Pending', colorClass: 'bg-warn-100 text-warn-800' });
        expect(container.querySelector('span')?.hasAttribute('data-test')).toBe(false);
    });

    it('appends extra class prop alongside base classes', () => {
        const { container } = render(StatusBadge, {
            label: 'Pending',
            colorClass: 'bg-warn-100 text-warn-800',
            class: 'ml-2'
        });
        expect(container.querySelector('span')?.className).toContain('ml-2');
    });
});
