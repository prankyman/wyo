import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ShellHeaderHarness from '$test/harnesses/shell/ShellHeaderTestHarness.svelte';

/**
 * The bar of the top of the shell: its left side, and the buttons in one row that never wraps and never shrinks, so they stay at
 * the top right at every width — a phone's 390 pixels included — while the left side gives way.
 */

describe('ShellHeader', () => {
    it('draws the left side of the shell beside the buttons', () => {
        render(ShellHeaderHarness);

        expect(screen.getByTestId('shell-header-left')).toContainElement(screen.getByTestId('harness-left'));
        expect(screen.getByTestId('shell-header-buttons')).toContainElement(screen.getByTestId('harness-button-1'));
    });

    it('keeps every button in the one row at the right, in order', () => {
        render(ShellHeaderHarness);

        const row = screen.getByTestId('shell-header-buttons');
        expect([...row.children].map((child) => child.getAttribute('data-test'))).toEqual([
            'harness-button-1',
            'harness-button-2',
            'harness-button-3'
        ]);
        expect(row).toHaveClass('flex', 'flex-nowrap', 'shrink-0');
        expect(row).not.toHaveClass('flex-wrap');
    });

    it('lets the left side shrink and never wraps the bar', () => {
        render(ShellHeaderHarness);

        expect(screen.getByTestId('shell-header-left')).toHaveClass('min-w-0');
        const bar = screen.getByTestId('shell-header-bar');
        expect(bar).toHaveClass('flex-nowrap', 'justify-between');
        expect(bar).not.toHaveClass('flex-wrap');
        expect([...bar.children]).toEqual([
            screen.getByTestId('shell-header-left'),
            screen.getByTestId('shell-header-buttons')
        ]);
    });
});
