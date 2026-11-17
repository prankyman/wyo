import { render } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import StepStrip from './StepStrip.svelte';
import { stageSteps, type IStepStripStep } from './step-strip';

// A language that exists only for this test, to prove the `lang` prop reaches the text system.
const TEST_LANGUAGE = vi.hoisted(() => ({
    test: {
        'stepStrip.label': 'Steps',
        'stepStrip.done': 'done',
        'stepStrip.current': 'current step',
        'stepStrip.todo': 'to do'
    }
}));
vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ texts: TEST_LANGUAGE }));
vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock({ texts: TEST_LANGUAGE }));

/**
 * The step strip: «1 Details · 2 Items · 3 Review · 4 Send · 5 Close» with the current step
 * marked and a line «Now: …» that says what to do next and who does it. A generic, display-only component: it reads no data and
 * its chips are not controls.
 */

const STEPS: IStepStripStep[] = [
    { key: 'details', label: 'Details', state: 'done' },
    { key: 'items', label: 'Items', state: 'current' },
    { key: 'review', label: 'Review', state: 'todo' },
    { key: 'send', label: 'Send', state: 'todo' },
    { key: 'close', label: 'Close', state: 'todo' }
];

const tokens = (element: Element): string[] => (element.getAttribute('class') ?? '').split(/\s+/).filter(Boolean);

describe('StepStrip', () => {
    it('lists the numbered steps in order, marks only the current one, and says what to do now', () => {
        const { container } = render(StepStrip, {
            props: {
                lang: 'ru',
                steps: STEPS,
                nowText: 'Now: fill in the items. The author does this.',
                dataTest: 'document-steps'
            }
        });

        const list = container.querySelector('ol[data-test="document-steps"]') as HTMLElement;
        expect(list).toHaveAttribute('aria-label', 'Шаги');
        const chips = [...list.querySelectorAll('li')];
        expect(chips.map((chip) => chip.getAttribute('data-test'))).toEqual([
            'document-steps-details',
            'document-steps-items',
            'document-steps-review',
            'document-steps-send',
            'document-steps-close'
        ]);
        expect(chips.map((chip) => chip.getAttribute('data-state'))).toEqual([
            'done',
            'current',
            'todo',
            'todo',
            'todo'
        ]);

        // The number and the label are what is seen; the state word is for a screen reader, so colour is not the only cue.
        expect(chips.map((chip) => chip.querySelector('[data-test$="-number"]')?.textContent?.trim())).toEqual([
            '1',
            '2',
            '3',
            '4',
            '5'
        ]);
        expect(chips.map((chip) => chip.querySelector('[data-test$="-label"]')?.textContent?.trim())).toEqual([
            'Details',
            'Items',
            'Review',
            'Send',
            'Close'
        ]);
        expect(chips.map((chip) => chip.querySelector('.sr-only')?.textContent?.trim())).toEqual([
            'выполнен',
            'текущий шаг',
            'впереди',
            'впереди',
            'впереди'
        ]);

        const current = container.querySelectorAll('[aria-current="step"]');
        expect(current).toHaveLength(1);
        expect(current[0]).toBe(chips[1]);

        // A finished step is told by a tick as well as by its colour; the tick is decoration for a screen reader.
        expect(chips.map((chip) => chip.querySelector('svg') !== null)).toEqual([true, false, false, false, false]);
        expect(chips[0].querySelector('svg')).toHaveAttribute('aria-hidden', 'true');

        expect(container.querySelector('[data-test="document-steps-now"]')).toHaveTextContent(
            'Now: fill in the items. The author does this.'
        );
    });

    it('puts the tour anchor on the whole strip only when the screen gives one', () => {
        const withTour = render(StepStrip, {
            props: {
                lang: 'ru',
                steps: STEPS,
                nowText: 'Now: scan.',
                dataTest: 'document-steps',
                dataTour: 'document-steps-tour'
            }
        });
        const anchored = withTour.container.querySelector('[data-tour="document-steps-tour"]') as HTMLElement;
        expect(anchored).toContainElement(
            withTour.container.querySelector('ol[data-test="document-steps"]') as HTMLElement
        );
        expect(anchored).toContainElement(
            withTour.container.querySelector('[data-test="document-steps-now"]') as HTMLElement
        );
        withTour.unmount();

        const without = render(StepStrip, {
            props: { lang: 'ru', steps: STEPS, nowText: 'Now: scan.', dataTest: 'document-steps' }
        });
        expect(without.container.querySelector('[data-tour]')).toBeNull();
    });

    it('is display-only: no link, button, form or field, and no sideways scrolling', () => {
        const { container } = render(StepStrip, {
            props: { lang: 'ru', steps: STEPS, nowText: 'Now: scan.', dataTest: 'document-steps' }
        });

        expect(container.querySelector('a, button, form, input, select, textarea, summary')).toBeNull();
        const classes = tokens(container.querySelector('ol') as HTMLElement);
        expect(classes).toContain('flex-wrap');
        expect(classes.filter((token) => token.startsWith('overflow-x'))).toEqual([]);
    });

    it('marks no step as current when none is, and draws no «Now» line for an empty text', () => {
        const finished = STEPS.map((step): IStepStripStep => ({ ...step, state: 'done' }));
        const { container } = render(StepStrip, {
            props: { lang: 'ru', steps: finished, nowText: '', dataTest: 'document-steps' }
        });

        expect(container.querySelector('[aria-current]')).toBeNull();
        expect(container.querySelector('[data-test="document-steps-now"]')).toBeNull();
        expect(container.querySelectorAll('li[data-state="done"]')).toHaveLength(5);
    });

    it('takes the language of its fixed words from the lang prop', () => {
        const { container } = render(StepStrip, {
            props: { lang: 'test', steps: STEPS.slice(0, 2), nowText: 'Now: scan.', dataTest: 'document-steps' }
        });

        expect(container.querySelector('ol')).toHaveAttribute('aria-label', 'Steps');
        const words = [...container.querySelectorAll('.sr-only')].map((word) => word.textContent?.trim());
        expect(words).toEqual(['done', 'current step']);
    });

    it('draws small chips without numbers and ticks in compact mode, only the current one filled', () => {
        const { container } = render(StepStrip, {
            props: { lang: 'ru', steps: STEPS, nowText: 'Now: scan.', dataTest: 'batch-strip', compact: true }
        });

        const chips = [...container.querySelectorAll('li')];
        expect(chips).toHaveLength(5);
        expect(container.querySelector('[data-test$="-number"]')).toBeNull();
        expect(container.querySelector('svg')).toBeNull();
        expect(chips.map((chip) => chip.getAttribute('data-state'))).toEqual([
            'done',
            'current',
            'todo',
            'todo',
            'todo'
        ]);
        expect(chips.filter((chip) => tokens(chip).includes('bg-brand-600'))).toEqual([chips[1]]);
        expect(chips[1]).toHaveAttribute('aria-current', 'step');
        expect(container.querySelector('[data-test="batch-strip-now"]')).toHaveTextContent('Now: scan.');
    });

    it('draws the steps as plain words joined by arrows in the words look, the current one in bold', () => {
        const { container } = render(StepStrip, {
            props: { lang: 'ru', steps: STEPS.slice(0, 3), dataTest: 'delivery-stages', look: 'words' }
        });

        const chips = [...container.querySelectorAll('li')];
        expect(chips.map((chip) => chip.querySelector('[data-test$="-label"]')?.textContent)).toEqual([
            'Details',
            '→ Items',
            '→ Review'
        ]);
        expect(container.querySelector('[data-test$="-number"]')).toBeNull();
        expect(container.querySelector('svg')).toBeNull();
        expect(tokens(chips[1])).toContain('font-medium');
        expect(tokens(chips[0])).not.toContain('font-medium');
        expect(chips[1]).toHaveAttribute('aria-current', 'step');
    });
});

describe('stageSteps', () => {
    const KEYS = ['draft', 'quoted', 'ready'] as const;
    const states = (current: (typeof KEYS)[number] | null): string[] =>
        stageSteps(KEYS, current, (key) => key.toUpperCase()).map((step) => step.state);

    it.each([
        ['draft', ['current', 'todo', 'todo']],
        ['quoted', ['done', 'current', 'todo']],
        ['ready', ['done', 'done', 'current']],
        [null, ['todo', 'todo', 'todo']]
    ] as const)('marks the stages before %s as done and the ones after as to do', (current, expected) => {
        expect(states(current)).toEqual(expected);
    });

    it('words every stage with the given function and keeps the stage as its key', () => {
        expect(stageSteps(KEYS, 'quoted', (key) => key.toUpperCase())[1]).toEqual({
            key: 'quoted',
            label: 'QUOTED',
            state: 'current'
        });
    });
});
