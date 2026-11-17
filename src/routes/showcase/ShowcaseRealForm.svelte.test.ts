import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import type { TApiResult } from '$lib/api/api';
import { messages } from '$lib/i18n/messages';
import { sendForm } from '$test/support/superform-answer';
import ShowcaseRealForm from './ShowcaseRealForm.svelte';

const text = (key: string): string => messages.ru[key];
const ok = async (): Promise<TApiResult<unknown>> => ({ ok: true, data: {} });

/** Types into the boxes the schema requires. */
async function fill(name: string, amount = '12,5'): Promise<void> {
    await fireEvent.input(screen.getByTestId('real-form-name'), { target: { value: name } });
    await fireEvent.change(screen.getByTestId('real-form-kind'), { target: { value: 'first' } });
    await fireEvent.input(screen.getByTestId('real-form-amount'), { target: { value: amount } });
}

describe('ShowcaseRealForm (the one real form of the showcase page)', () => {
    it('checks the form in the browser with its schema, and sends nothing while a box is refused', async () => {
        const send = vi.fn(ok);
        render(ShowcaseRealForm, { props: { send } });

        await fill('a');
        await sendForm(screen.getByTestId('real-form'));

        await waitFor(() =>
            expect(screen.getByTestId('real-form-name-field-error')).toHaveTextContent(text('errors.invalid'))
        );
        expect(send).not.toHaveBeenCalled();
    });

    it('sends the typed values as they are and draws «Сохранено.» under the button', async () => {
        const send = vi.fn(ok);
        render(ShowcaseRealForm, { props: { send } });

        await fill('Вася');
        await sendForm(screen.getByTestId('real-form'));

        await waitFor(() => expect(screen.getByTestId('real-form-answer-saved')).toBeInTheDocument());
        expect(screen.getByTestId('real-form-answer-saved')).toHaveTextContent(text('showcase.forms.answer.saved'));
        expect(send).toHaveBeenCalledExactlyOnceWith(
            expect.objectContaining({ name: 'Вася', kind: 'first', amount: '12.5' })
        );
    });

    it('lays the refusal of the stub that stands in for the wyo server onto the name box and under the button', async () => {
        render(ShowcaseRealForm);

        await fill('отказ');
        await sendForm(screen.getByTestId('real-form'));

        await waitFor(() => expect(screen.getByTestId('real-form-answer-error')).toBeInTheDocument());
        expect(screen.getByTestId('real-form-answer-error')).toHaveTextContent(text('errors.invalid'));
        expect(screen.getByTestId('real-form-name-field-error')).toHaveTextContent(text('errors.invalid'));
    });
});
