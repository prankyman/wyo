import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import JournalOutcomeBadge from './JournalOutcomeBadge.svelte';

/** How one request to a marketplace ended, in one of six words, each in its tone and with its «?». */
describe('JournalOutcomeBadge', () => {
    it.each([
        ['done', 'Выполнен', 'bg-ok-50'],
        ['limited', 'Предел запросов', 'bg-warn-50'],
        ['refused', 'Отказ', 'bg-stop-50'],
        ['server_error', 'Ошибка сервера', 'bg-stop-50'],
        ['no_connection', 'Нет связи', 'bg-warn-50'],
        ['key_refused', 'Ключ не принят', 'bg-stop-50']
    ] as const)('draws %s as «%s»', (outcome, label, toneClass) => {
        render(JournalOutcomeBadge, { props: { lang: 'ru', outcome, testId: 'outcome' } });

        expect(screen.getByTestId('outcome')).toHaveTextContent(label);
        expect(screen.getByTestId('outcome')).toHaveClass(toneClass);
        expect(screen.getByTestId('outcome-help-button')).toBeInTheDocument();
    });
});
