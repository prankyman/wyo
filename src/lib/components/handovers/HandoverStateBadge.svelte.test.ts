import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import HandoverStateBadge from './HandoverStateBadge.svelte';

/** The state of a hand-over in the tone of the state, with a «?» that says what it means and what the owner does. */
describe('HandoverStateBadge', () => {
    it.each([
        ['working', 'идёт', 'bg-wait-50'],
        ['waiting', 'ждёт вас', 'bg-warn-50'],
        ['problem', 'проблема', 'bg-stop-50'],
        ['documents_sent', 'документы отправлены', 'bg-ok-50'],
        ['withdrawn', 'отозвана', 'bg-ink-100'],
        ['completed', 'завершена', 'bg-ok-50']
    ] as const)('draws %s as «%s»', (state, label, toneClass) => {
        render(HandoverStateBadge, { props: { lang: 'ru', state, testId: 'handover-state' } });

        expect(screen.getByTestId('handover-state')).toHaveTextContent(label);
        expect(screen.getByTestId('handover-state')).toHaveClass(toneClass);
        expect(screen.getByTestId('handover-state-help-button')).toBeInTheDocument();
    });
});
