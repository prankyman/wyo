import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import Stack from './Stack.svelte';

const block = createRawSnippet(() => ({
    render: () => `<p data-test="stack-block">The first block</p>`
}));

describe('Stack', () => {
    it.each(['div', 'section'] as const)(
        'draws its blocks in one column with the one 16 px gap, as a %s marked for the browser measure',
        (as) => {
            render(Stack, { props: { as, dataTest: 'stack', children: block } });

            const stack = screen.getByTestId('stack');
            expect(stack.tagName).toBe(as.toUpperCase());
            expect(stack).toHaveClass('flex', 'flex-col', 'gap-4');
            expect(stack).toHaveAttribute('data-stack');
            expect(stack).toContainElement(screen.getByTestId('stack-block'));
        }
    );

    it('is a div by default and carries no id, test id or tour anchor it was not given', () => {
        const { container } = render(Stack, { props: { children: block } });

        const stack = container.querySelector('[data-stack]');
        expect(stack?.tagName).toBe('DIV');
        expect(stack).not.toHaveAttribute('id');
        expect(stack).not.toHaveAttribute('data-test');
        expect(stack).not.toHaveAttribute('data-tour');
    });

    it('takes an id, a tour anchor, the classes of its width and any other attribute of its state', () => {
        render(Stack, {
            props: {
                id: 'client-card',
                dataTest: 'stack',
                dataTour: 'client-card',
                class: 'w-full',
                'data-state': 'open',
                children: block
            }
        });

        const stack = screen.getByTestId('stack');
        expect(stack).toHaveAttribute('id', 'client-card');
        expect(stack).toHaveAttribute('data-tour', 'client-card');
        expect(stack).toHaveAttribute('data-state', 'open');
        expect(stack).toHaveClass('w-full', 'gap-4');
    });
});
