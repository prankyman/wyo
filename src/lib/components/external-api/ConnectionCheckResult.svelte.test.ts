import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import { connectionOf } from '$test/support/external-api';
import ConnectionCheckResult from './ConnectionCheckResult.svelte';
import type { TExternalApi } from './types';

function draw(connection: TExternalApi): void {
    render(ConnectionCheckResult, { props: { lang: 'ru', connection, testId: 'result' } });
}

/** What the fulfilment centre's system answered at the last check: the state in its tone, the seller's name, the key's end, the marketplaces. */
describe('ConnectionCheckResult', () => {
    it('shows the state, the address, the name the fulfilment centre gives the seller, the end of the key, the marketplaces it accepts and when it was checked', () => {
        draw(connectionOf());

        expect(screen.getByTestId('result-state')).toHaveTextContent('Работает');
        expect(screen.getByTestId('result-state')).toHaveClass('bg-ok-50');
        expect(screen.getByTestId('result-state-help-button')).toBeInTheDocument();
        expect(screen.getByTestId('result-facts-address')).toHaveTextContent('https://fc.example.com/api');
        expect(screen.getByTestId('result-facts-seller')).toHaveTextContent('ИП Иванов (склад на Лесной)');
        expect(screen.getByTestId('result-facts-key')).toHaveTextContent('заканчивается на k9z2');
        expect(screen.getByTestId('result-facts-marketplaces')).toHaveTextContent('Маркетплейс А, Маркетплейс Б');
        expect(screen.getByTestId('result-facts-checkedAt')).toHaveTextContent('18 нояб. 2026');
    });

    it.each([
        ['key_refused', 'Ключ не принят', 'bg-stop-50'],
        ['read_only_key', 'Ключ только читает', 'bg-stop-50'],
        ['unavailable', 'Система недоступна', 'bg-warn-50'],
        ['no_connection', 'Нет связи', 'bg-warn-50']
    ] as const)('draws the state %s as «%s» in its tone', (state, label, toneClass) => {
        draw(connectionOf({ state }));

        expect(screen.getByTestId('result-state')).toHaveTextContent(label);
        expect(screen.getByTestId('result-state')).toHaveClass(toneClass);
    });

    it('leaves out what the fulfilment centre did not tell, and says it accepts no marketplace when its list is empty', () => {
        draw(connectionOf({ sellerName: undefined, checkedAt: undefined, marketplaces: [] }));

        expect(screen.queryByTestId('result-facts-seller')).toBeNull();
        expect(screen.queryByTestId('result-facts-checkedAt')).toBeNull();
        expect(screen.getByTestId('result-facts-marketplaces')).toHaveTextContent('ни один маркетплейс');
    });

    it('draws only the state for a setting that is not filled', () => {
        draw({ state: 'empty', eventsMode: 'poll' });

        expect(screen.getByTestId('result-state')).toHaveTextContent('Не заполнено');
        expect(screen.queryByTestId('result-facts')).toBeNull();
    });
});
