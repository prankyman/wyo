import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { resetPage, setNavigating } from '$test/support/page-state.svelte';
import Button from './Button.svelte';
import { pageChange } from './page-change.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

const text = (value: string) => createRawSnippet(() => ({ render: () => `<span>${value}</span>` }));
const icon = createRawSnippet(() => ({ render: () => '<svg data-test="button-icon"></svg>' }));

afterEach(() => {
    pageChange.lockExpired = false;
    resetPage();
});

describe('Button', () => {
    it('is a primary 44 px button with a pressed state, a focus ring, and runs its onclick', async () => {
        const onclick = vi.fn();
        render(Button, { children: text('Save'), onclick, dataTest: 'save' });

        const button = screen.getByTestId('save');
        expect(button.tagName).toBe('BUTTON');
        expect(button).toHaveAttribute('type', 'button');
        expect(button).toHaveTextContent('Save');
        // The press feedback and the keyboard ring are the rendering contract (jsdom applies no CSS).
        expect(button).toHaveClass(
            'bg-brand-600',
            'pressable:active:bg-brand-800',
            'motion-safe:pressable:active:scale-[0.97]',
            'touch-manipulation',
            'min-h-11',
            'focus-visible:outline-2'
        );

        await fireEvent.click(button);
        expect(onclick).toHaveBeenCalledTimes(1);
    });

    it.each([
        ['primary', 'bg-brand-600'],
        ['secondary', 'border-ink-300'],
        ['danger', 'bg-stop-700'],
        ['danger-outline', 'border-stop-300'],
        ['success', 'bg-ok-700'],
        ['warning', 'border-warn-400'],
        ['ghost', 'pressable:hover:bg-ink-100'],
        ['ghost-inverse', 'text-white'],
        ['link', 'underline']
    ] as const)('draws the %s variant with %s', (variant, expected) => {
        render(Button, { variant, children: text(variant), dataTest: 'b' });

        expect(screen.getByTestId('b')).toHaveClass(expected);
    });

    it.each([
        ['md', 'min-h-11', 'min-w-11'],
        ['lg', 'min-h-14', 'min-w-14'],
        ['xl', 'min-h-16', 'min-w-16']
    ] as const)('is %s tall, and square when it is only an icon', (size, height, width) => {
        render(Button, { size, icon, ariaLabel: 'Camera', dataTest: 'b' });

        const button = screen.getByRole('button', { name: 'Camera' });
        expect(button).toHaveClass(height, width);
        expect(screen.getByTestId('button-icon')).toBeInTheDocument();
    });

    it('passes the form attributes through and lets an extra class win over the variant', () => {
        render(Button, {
            type: 'submit',
            name: 'intent',
            value: 'release',
            form: 'orders-form',
            formaction: '?/releaseAll',
            id: 'release-all',
            fullWidth: true,
            class: 'bg-warn-600',
            'aria-describedby': 'why',
            children: text('Release'),
            dataTest: 'b'
        });

        const button = screen.getByTestId('b');
        expect(button).toHaveAttribute('type', 'submit');
        expect(button).toHaveAttribute('name', 'intent');
        expect(button).toHaveAttribute('value', 'release');
        expect(button).toHaveAttribute('form', 'orders-form');
        expect(button).toHaveAttribute('formaction', '?/releaseAll');
        expect(button).toHaveAttribute('id', 'release-all');
        expect(button).toHaveAttribute('aria-describedby', 'why');
        expect(button).toHaveClass('w-full', 'bg-warn-600');
        expect(button).not.toHaveClass('bg-ink-900');
    });

    it.each([
        'primary',
        'secondary',
        'danger',
        'danger-outline',
        'success',
        'warning',
        'ghost',
        'ghost-inverse',
        'link'
    ] as const)(
        'draws the %s variant with no hover or pressed look a greyed-out or busy button could show',
        (variant) => {
            render(Button, { variant, children: text(variant), dataTest: 'b' });

            // `pressable:` (src/app.css) holds them back on a `disabled` or `aria-disabled` button;
            // an unguarded `hover:`/`active:` would still darken or shrink it (jsdom applies no CSS).
            const classes = [...screen.getByTestId('b').classList];
            const looks = classes.filter((name) => /(^|:)(hover|active):/.test(name));
            expect(looks.length).toBeGreaterThan(0);
            expect(looks.filter((name) => !name.includes('pressable:'))).toEqual([]);
        }
    );

    it("lets an extra hover look replace the variant's, both held back on a button that cannot act", () => {
        render(Button, {
            variant: 'ghost',
            class: 'pressable:hover:bg-transparent',
            children: text('Reset'),
            dataTest: 'b'
        });

        const button = screen.getByTestId('b');
        expect(button).toHaveClass('pressable:hover:bg-transparent');
        expect(button).not.toHaveClass('pressable:hover:bg-ink-100');
    });

    it('while pending shows a spinner, says it is busy, and cannot fire again — without becoming disabled', async () => {
        const onclick = vi.fn();
        render(Button, { pending: true, icon, onclick, children: text('Complete'), dataTest: 'b' });

        const button = screen.getByTestId('b');
        expect(screen.getByTestId('button-spinner')).toBeInTheDocument();
        // The spinner takes the icon's place instead of pushing the text aside.
        expect(screen.queryByTestId('button-icon')).toBeNull();
        expect(button).toHaveAttribute('aria-busy', 'true');
        expect(button).toHaveAttribute('aria-disabled', 'true');
        expect(button).not.toBeDisabled();

        await fireEvent.click(button);
        expect(onclick).not.toHaveBeenCalled();
    });

    it('is really disabled when told so, and then shows no spinner even if pending', () => {
        render(Button, { disabled: true, pending: true, children: text('Close'), dataTest: 'b' });

        const button = screen.getByTestId('b');
        expect(button).toBeDisabled();
        expect(screen.queryByTestId('button-spinner')).toBeNull();
        expect(button).not.toHaveAttribute('aria-busy');
    });

    it('as a link shows it is working while the application is on its way to its address, and swallows a second press', async () => {
        const onclick = vi.fn();
        render(Button, { href: '/orders/new', onclick, children: text('New order'), dataTest: 'b' });

        const link = screen.getByTestId('b');
        expect(link.tagName).toBe('A');
        expect(link).toHaveAttribute('href', '/orders/new');
        expect(link).not.toHaveAttribute('aria-busy');

        setNavigating('/orders/new');
        await Promise.resolve();

        expect(link).toHaveAttribute('aria-busy', 'true');
        expect(screen.getByTestId('button-spinner')).toBeInTheDocument();
        const click = new MouseEvent('click', { bubbles: true, cancelable: true });
        link.dispatchEvent(click);
        expect(click.defaultPrevented).toBe(true);
        expect(onclick).not.toHaveBeenCalled();
    });

    it('as a link stops showing it is working and lets a press through once the page change is taken as dropped', async () => {
        const onclick = vi.fn();
        render(Button, { href: '/orders/new', onclick, children: text('New order'), dataTest: 'b' });
        const link = screen.getByTestId('b');
        setNavigating('/orders/new');
        await Promise.resolve();
        expect(link).toHaveAttribute('aria-busy', 'true');

        // The page-change guard sets this after 10 s of a change that does not end.
        pageChange.lockExpired = true;
        await Promise.resolve();

        expect(link).not.toHaveAttribute('aria-busy');
        expect(link).not.toHaveAttribute('aria-disabled');
        expect(screen.queryByTestId('button-spinner')).toBeNull();
        // The test setup cancels every link click that reaches the document, so the press that went
        // through is told by its own handler running, which a swallowed press never reaches.
        link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
        expect(onclick).toHaveBeenCalledTimes(1);
    });

    it('as a download or new-window link never waits, since the page stays where it is', async () => {
        render(Button, { href: '/orders/export?format=csv', download: true, children: text('CSV'), dataTest: 'b' });

        setNavigating('/orders/export?format=csv');
        await Promise.resolve();

        const link = screen.getByTestId('b');
        expect(link).toHaveAttribute('download', '');
        expect(link).not.toHaveAttribute('aria-busy');
        expect(screen.queryByTestId('button-spinner')).toBeNull();
    });

    it('as a disabled link has no address to follow', () => {
        // `target` is also a Svelte mount option, so the props go under `props`.
        render(Button, {
            props: {
                href: '/print',
                target: '_blank',
                rel: 'noopener',
                disabled: true,
                children: text('Print'),
                dataTest: 'b'
            }
        });

        const link = screen.getByTestId('b');
        expect(link).not.toHaveAttribute('href');
        expect(link).toHaveAttribute('aria-disabled', 'true');
        expect(link).toHaveAttribute('target', '_blank');
        expect(link).toHaveAttribute('rel', 'noopener');
    });
});
