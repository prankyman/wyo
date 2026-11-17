import { flushTicks } from '$test/support/superform-answer';
import SectionFormTestHarness from '$test/harnesses/ui/SectionFormTestHarness.svelte';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { get, writable } from 'svelte/store';
import { describe, expect, it, vi } from 'vitest';

/**
 * One folded section of an edit screen as one form of its own: «Сохранить» only after a change,
 * «Сохранено» and a folded section after a successful save, a question before a change is dropped, and the create screen's
 * section that is only a part of the screen's one form.
 */

vi.mock('$app/forms', () => ({ enhance: () => ({ destroy: () => undefined }) }));

function standIn(values: Record<string, unknown> = { legalName: 'ООО «Ромашка»' }) {
    const form = writable({ ...values });
    const tainted = writable<Record<string, boolean> | undefined>(undefined);
    const message = writable<unknown>(undefined);
    const reset = vi.fn((options?: { newState?: Record<string, unknown> }) => {
        form.set({ ...(options?.newState ?? values) });
        tainted.set(undefined);
    });
    return {
        enhance: () => ({ destroy: () => undefined }),
        submitting: writable(false),
        message,
        tainted,
        form,
        isTainted: (state?: unknown) => Boolean(state && Object.keys(state as object).length > 0),
        reset,
        typed(value: string) {
            form.update((current) => ({ ...current, legalName: value }));
            tainted.set({ legalName: true });
        }
    };
}

async function openSection(): Promise<HTMLDetailsElement> {
    const section = screen.getByTestId('client-section-legal') as HTMLDetailsElement;
    section.open = true;
    await fireEvent(section, new Event('toggle'));
    await flushTicks();
    return section;
}

async function type(value: string): Promise<void> {
    const box = screen.getByTestId('legal-name') as HTMLInputElement;
    box.value = value;
    await fireEvent.input(box);
    await flushTicks();
}

describe('SectionForm', () => {
    it('is a form of its own, with «Закрыть» and no «Сохранить» until a field changes', async () => {
        render(SectionFormTestHarness, { props: { form: standIn({ legalName: 'ООО «Ромашка»', drawn: 'fp-1' }) } });
        const section = await openSection();
        const form = screen.getByTestId('client-section-legal-form');
        expect(form.tagName).toBe('FORM');
        // The fingerprint of the values the section was drawn with is posted, so the wyo server's stale check compares it.
        expect((form.querySelector('input[name="drawn"]') as HTMLInputElement).value).toBe('fp-1');
        expect(section.contains(form)).toBe(true);
        expect(screen.queryByTestId('client-section-legal-save')).toBeNull();
        expect(screen.getByTestId('client-section-legal-close')).toHaveTextContent('Закрыть');

        await type('ООО «Василёк»');
        expect(screen.getByTestId('client-section-legal-save')).toHaveTextContent('Сохранить');
        expect(screen.getByTestId('client-section-legal-save').closest('[data-form-buttons]')).not.toBeNull();
    });

    it('says «Сохранено» in the heading row and folds the section after a successful save', async () => {
        const form = standIn();
        render(SectionFormTestHarness, { props: { form } });
        const section = await openSection();
        await type('ООО «Василёк»');

        form.tainted.set(undefined);
        form.message.set({ saved: 'section' });
        await flushTicks();
        expect(section.open).toBe(false);
        expect(screen.getByTestId('client-section-legal-saved')).toHaveTextContent('Сохранено.');
        expect(screen.getByTestId('client-section-legal-heading')).toContainElement(
            screen.getByTestId('client-section-legal-saved')
        );
    });

    it('draws a refusal under its buttons and stays open', async () => {
        const form = standIn();
        render(SectionFormTestHarness, { props: { form } });
        const section = await openSection();
        await type('ООО «Василёк»');

        form.message.set({ error: 'app.forms.section.otherRefused', params: { section: 'Основное', reason: 'x' } });
        await flushTicks();
        expect(section.open).toBe(true);
        expect(screen.getByTestId('client-section-legal-error')).toBeInTheDocument();
    });

    it('puts the cursor in the box a refusal of the server names', async () => {
        const form = standIn();
        const { rerender } = render(SectionFormTestHarness, { props: { form } });
        await openSection();
        await type('ООО «Василёк»');
        (document.activeElement as HTMLElement | null)?.blur();

        await rerender({ form, innError: 'ИНН — 10 или 12 цифр.' });
        form.message.set({ error: 'app.clients.errors.inn' });
        await flushTicks();
        expect(screen.getByTestId('inn')).toHaveFocus();
    });

    it('folds at once when nothing changed, and asks first when something did; «Закрыть без сохранения» puts the stored values back', async () => {
        const form = standIn();
        render(SectionFormTestHarness, { props: { form } });
        let section = await openSection();
        await fireEvent.click(screen.getByTestId('client-section-legal-close'));
        await flushTicks();
        expect(section.open).toBe(false);
        expect(screen.queryByTestId('client-section-legal-unsaved')).toBeNull();

        section = await openSection();
        await type('ООО «Василёк»');
        await fireEvent.click(screen.getByTestId('client-section-legal-close'));
        await flushTicks();
        const question = screen.getByTestId('client-section-legal-unsaved');
        expect(question).toHaveTextContent('Изменения не сохранены. Закрыть без сохранения?');
        expect(section.open).toBe(true);

        await fireEvent.click(screen.getByText('Закрыть без сохранения'));
        await flushTicks();
        expect(form.reset).toHaveBeenCalledWith({ newState: { legalName: 'ООО «Ромашка»' } });
        expect(get(form.form)).toEqual({ legalName: 'ООО «Ромашка»' });
        expect(section.open).toBe(false);
    });

    it('offers no «Сохранить» to a reader who may not write', async () => {
        render(SectionFormTestHarness, { props: { form: standIn(), canWrite: false } });
        await openSection();
        await type('ООО «Василёк»');
        expect(screen.queryByTestId('client-section-legal-save')).toBeNull();
        expect(screen.getByTestId('client-section-legal-close')).toBeInTheDocument();
    });

    it('on a create screen is an open part of the one form, with no form and no buttons of its own', () => {
        render(SectionFormTestHarness, { props: { form: standIn(), create: true } });
        expect((screen.getByTestId('client-section-legal') as HTMLDetailsElement).open).toBe(true);
        expect(screen.queryByTestId('client-section-legal-form')).toBeNull();
        expect(screen.queryByTestId('client-section-legal-close')).toBeNull();
        expect(screen.getByTestId('legal-name')).toBeInTheDocument();
    });

    it.each([
        ['draws no space around itself unless the screen asks for one', undefined, false],
        ['takes the classes of its outer box from the screen', 'mb-4', true]
    ])('%s', (_name, outerClass, hasSpace) => {
        render(SectionFormTestHarness, { props: { form: standIn(), class: outerClass } });
        expect(screen.getByTestId('client-section-legal').classList.contains('mb-4')).toBe(hasSpace);
    });
});
