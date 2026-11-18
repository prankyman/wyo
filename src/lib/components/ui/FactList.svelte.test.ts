import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import FactList from './FactList.svelte';

/** The facts of a record as a list of names and values, each name with its «?» when it has one. */
describe('FactList', () => {
    it('draws every fact with its name and value, a «?» beside a name that has a help text and none beside one without', () => {
        render(FactList, {
            props: {
                dataTest: 'facts',
                items: [
                    { key: 'seller', label: 'Продавец', value: 'ИП Иванов', help: 'Имя продавца.' },
                    { key: 'key', label: 'Ключ', value: 'заканчивается на a1b2' }
                ]
            }
        });

        const list = screen.getByTestId('facts');
        expect(list.tagName).toBe('DL');
        expect(
            within(list)
                .getAllByRole('term')
                .map((term) => term.textContent?.trim())
        ).toEqual(['Продавец', 'Ключ']);
        expect(screen.getByTestId('facts-seller')).toHaveTextContent('ИП Иванов');
        expect(screen.getByTestId('facts-key')).toHaveTextContent('заканчивается на a1b2');
        expect(screen.getByTestId('facts-seller-help-button')).toHaveAccessibleName('Имя продавца.');
        expect(screen.queryByTestId('facts-key-help-button')).toBeNull();
    });
});
