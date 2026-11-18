import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import SectionLoading from './SectionLoading.svelte';

function draw(refusal: string | null, onRetry = vi.fn()): typeof onRetry {
    render(SectionLoading, {
        props: { lang: 'ru', id: 'external-api-warehouses', title: 'Склады', help: 'Что это.', refusal, onRetry }
    });
    return onRetry;
}

/** The place of a section of «Внешний API» whose data has not come yet: its title, and while it waits «Читаем…», or the refusal with «Повторить». */
describe('SectionLoading', () => {
    it('draws the section’s title folded, and says it is reading while no refusal came', () => {
        draw(null);

        expect((screen.getByTestId('external-api-warehouses') as HTMLDetailsElement).open).toBe(false);
        expect(screen.getByTestId('external-api-warehouses-heading')).toHaveTextContent('Склады');
        expect(screen.getByTestId('external-api-warehouses-loading')).toHaveTextContent('Читаем');
        expect(screen.queryByTestId('external-api-warehouses-retry')).toBeNull();
    });

    it('words the refusal and asks again on «Повторить»', async () => {
        const onRetry = draw('internal');

        expect(screen.getByTestId('external-api-warehouses-refusal')).toHaveTextContent(
            'В программе wyo произошла ошибка.'
        );
        await fireEvent.click(screen.getByTestId('external-api-warehouses-retry'));

        expect(onRetry).toHaveBeenCalledOnce();
    });
});
