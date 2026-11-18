import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import InstallationStateCard from './InstallationStateCard.svelte';

const BASE = {
    testId: 'card-version',
    title: 'Версия программы',
    help: 'Здесь видно, какая версия установлена.',
    state: { label: 'Актуальная', tone: 'ok' as const },
    sentence: 'Установлена версия 1.2.3.'
};

describe('InstallationStateCard', () => {
    it('draws the title with its «?», the state in the colour of its tone, the sentence and the action as a link', () => {
        render(InstallationStateCard, {
            props: { ...BASE, action: { label: 'Открыть раздел «Настройки»', href: '/settings' } }
        });

        const card = screen.getByTestId('card-version');
        expect(within(card).getByRole('heading', { name: /Версия программы/ })).toBeInTheDocument();
        expect(within(card).getAllByTestId('card-version-help-button')).toHaveLength(1);
        expect(within(card).getByTestId('card-version-state')).toHaveTextContent('Актуальная');
        expect(within(card).getByTestId('card-version-state')).toHaveClass('bg-ok-50', 'text-ok-800');
        expect(within(card).getByTestId('card-version-sentence')).toHaveTextContent('Установлена версия 1.2.3.');
        const action = within(card).getByTestId('card-version-action');
        expect(action).toHaveAttribute('href', '/settings');
        expect(action).toHaveTextContent('Открыть раздел «Настройки»');
    });

    it.each([
        ['stop', 'bg-stop-50'],
        ['warn', 'bg-warn-50'],
        ['wait', 'bg-wait-50'],
        ['off', 'bg-ink-100']
    ] as const)('draws a %s state in its own colour', (tone, colour) => {
        render(InstallationStateCard, { props: { ...BASE, state: { label: 'Состояние', tone } } });

        expect(screen.getByTestId('card-version-state')).toHaveClass(colour);
    });

    it('draws no action when the card has none', () => {
        render(InstallationStateCard, { props: BASE });

        expect(screen.queryByTestId('card-version-action')).toBeNull();
    });
});
