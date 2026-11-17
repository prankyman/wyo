import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import FieldRefusal from './FieldRefusal.svelte';

const control = createRawSnippet(() => ({ render: () => '<input data-test="control" />' }));

describe('FieldRefusal', () => {
    it('draws the control alone while there is no refusal', () => {
        render(FieldRefusal, { props: { testId: 'f-error', children: control } });

        expect(screen.getByTestId('control')).toBeInTheDocument();
        expect(screen.queryByTestId('f-error')).toBeNull();
        expect(screen.getByTestId('f-error-box')).not.toHaveAttribute('data-field-error');
    });

    it('draws the refusal under the control and marks the box, so a closed section around it opens', () => {
        render(FieldRefusal, { props: { error: 'Choose a product', testId: 'f-error', children: control } });

        expect(screen.getByTestId('f-error')).toHaveTextContent('Choose a product');
        expect(screen.getByTestId('f-error')).toHaveAttribute('role', 'alert');
        expect(screen.getByTestId('f-error-box')).toHaveAttribute('data-field-error');
    });

    it('takes the classes the page gives for the field’s place in a grid', () => {
        render(FieldRefusal, { props: { testId: 'f-error', class: 'md:col-span-2', children: control } });

        expect(screen.getByTestId('f-error-box')).toHaveClass('md:col-span-2');
    });
});
