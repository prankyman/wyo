import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import CodeVerdictList from './CodeVerdictList.svelte';
import type { TCodeVerdict } from './types';

function draw(verdicts: TCodeVerdict[]): void {
    render(CodeVerdictList, { props: { lang: 'ru', verdicts, dataTest: 'verdicts' } });
}

/** The marketplace's verdict on each identifier of an order: its name, its verdict in a tone, the number of units and its own words. */
describe('CodeVerdictList', () => {
    it.each([
        ['accepted', 'принят', 'bg-ok-50'],
        ['not_required', 'проверка не нужна', 'bg-ink-100'],
        ['pending', 'проверяется', 'bg-wait-50'],
        ['unfinished', 'проверка не завершена', 'bg-warn-50'],
        ['missing', 'кода нет', 'bg-stop-50'],
        ['refused', 'не принят', 'bg-stop-50']
    ] as const)('draws the verdict %s as «%s»', (verdict, label, toneClass) => {
        draw([{ identifier: 'marking', verdict }]);

        const row = screen.getByTestId('verdicts-row');
        expect(row).toHaveTextContent('Код маркировки');
        expect(screen.getByTestId('verdicts-verdict')).toHaveTextContent(label);
        expect(screen.getByTestId('verdicts-verdict')).toHaveClass(toneClass);
        expect(screen.getByTestId('verdicts-verdict-help-button')).toBeInTheDocument();
    });

    it('names the units and the marketplace’s own words for the verdict, and never draws a code', () => {
        draw([
            { identifier: 'marking', verdict: 'refused', units: 2, words: 'Код не найден в обороте' },
            { identifier: 'imei', verdict: 'missing' }
        ]);

        const rows = screen.getAllByTestId('verdicts-row');
        expect(rows).toHaveLength(2);
        expect(rows[0]).toHaveTextContent('Отправлено единиц товара: 2');
        expect(rows[0]).toHaveTextContent('Слова маркетплейса: «Код не найден в обороте»');
        expect(rows[1]).toHaveTextContent('Номер IMEI');
        expect(rows[1]).not.toHaveTextContent('Отправлено');
    });
});
