import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import FieldMessage from './FieldMessage.svelte';

vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());

/** A box's refusal under the box. */
describe('FieldMessage', () => {
    it('words the first message of the field as an alert', () => {
        render(FieldMessage, {
            props: { lang: 'ru', messages: ['common.close', 'common.requestFailed'], dataTest: 'm' }
        });
        expect(screen.getByTestId('m')).toHaveAttribute('role', 'alert');
        expect(screen.getByTestId('m')).toHaveTextContent('Закрыть');
    });

    it('draws nothing for an accepted box', () => {
        render(FieldMessage, { props: { lang: 'ru', messages: undefined, dataTest: 'm' } });
        expect(screen.queryByTestId('m')).toBeNull();
        render(FieldMessage, { props: { lang: 'ru', messages: [], dataTest: 'm' } });
        expect(screen.queryByTestId('m')).toBeNull();
    });
});
