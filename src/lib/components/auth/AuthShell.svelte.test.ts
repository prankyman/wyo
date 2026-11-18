import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import AuthShell from './AuthShell.svelte';

const children = createRawSnippet(() => ({ render: () => '<form data-test="shell-body"></form>' }));

describe('AuthShell', () => {
    it('shows the program’s name over the page and renders the page body beneath it, never wrapping it in a form', () => {
        const { container } = render(AuthShell, { props: { children } });

        const brand = screen.getByTestId('auth-brand');
        expect(brand).toHaveTextContent('wyo');
        // The body follows the name in document order: the shell frames it, never wraps it in a form.
        const body = screen.getByTestId('shell-body');
        expect(brand.compareDocumentPosition(body) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        expect(container.querySelectorAll('form')).toHaveLength(1);
    });
});
