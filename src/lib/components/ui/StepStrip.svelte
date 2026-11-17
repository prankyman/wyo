<script lang="ts">
    import CheckIcon from 'phosphor-svelte/lib/CheckIcon';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { IStepStripStep, TStepStripState } from './step-strip';

    /**
     * The step strip: the steps of a document as numbered chips — «1 Данные · 2 Позиции ·
     * 3 Проверка · 4 Отправка · 5 Закрыть» on a document that goes through stages — with the current step
     * marked, and under them the line «Сейчас: …» that says what to do next and who does it.
     *
     * - Generic and display-only: the page hands it the steps with their state and the finished «Сейчас» line; it reads no
     *   data. The chips are NOT controls (no link, no button), so they carry no tap-target size; the current one has
     *   `aria-current="step"`.
     * - Phone-first: the chips wrap onto the next row instead of scrolling sideways.
     * - A state is never colour alone: a done chip shows a tick after its label, the current chip is filled with the brand colour, and each chip
     *   carries a state word for a screen reader (`stepStrip.*`). The number stays on every chip.
     * - `compact`: small chips without the numbers and the ticks, only the current one marked — the stages of a
     *   batch. `look="words"`: the stages as plain words joined by arrows, the current one in bold — the stages of a
     *   record inside a card's line.
     */
    interface IProps {
        /** For the fixed words: the list's name and the state of each chip. */
        lang: TLocale;
        steps: IStepStripStep[];
        /** The whole line «Сейчас: …», already translated; nothing is drawn when it is empty. */
        nowText?: string;
        dataTest: string;
        /** The `data-tour` anchor a learning step points at, when there is one (on the whole strip). */
        dataTour?: string;
        /** Small chips without numbers and ticks. */
        compact?: boolean;
        /** `chips`: boxed steps (the default); `words`: plain words joined by arrows, without numbers and ticks. */
        look?: 'chips' | 'words';
    }

    let { lang, steps, nowText = '', dataTest, dataTour, compact = false, look = 'chips' }: IProps = $props();

    const words = $derived(look === 'words');
    const plain = $derived(compact || words);

    const CHIP_CLASS: Record<TStepStripState, string> = {
        done: 'border-ok-300 bg-ok-50 text-ok-900',
        current: 'border-brand-600 bg-brand-600 text-white',
        todo: 'border-ink-300 bg-white text-ink-600'
    };
    const COMPACT_CHIP_CLASS: Record<TStepStripState, string> = {
        done: 'border-ink-300 text-ink-600',
        current: 'border-brand-600 bg-brand-600 text-white',
        todo: 'border-ink-300 text-ink-600'
    };
    const WORD_CLASS: Record<TStepStripState, string> = {
        done: 'text-ink-500',
        current: 'font-medium text-brand-700',
        todo: 'text-ink-500'
    };
    const NUMBER_CLASS: Record<TStepStripState, string> = {
        done: 'bg-ok-700 text-white',
        current: 'bg-white text-brand-700',
        todo: 'bg-ink-200 text-ink-700'
    };
    const STATE_KEY: Record<TStepStripState, string> = {
        done: 'stepStrip.done',
        current: 'stepStrip.current',
        todo: 'stepStrip.todo'
    };
</script>

<div data-tour={dataTour}>
    <ol
        class={words
            ? 'flex flex-wrap gap-x-2 gap-y-1 text-xs'
            : compact
              ? 'flex flex-wrap items-center gap-2 text-xs'
              : 'flex flex-wrap gap-2'}
        aria-label={t('stepStrip.label', lang)}
        data-test={dataTest}
    >
        {#each steps as step, index (step.key)}
            <li
                class={words
                    ? WORD_CLASS[step.state]
                    : compact
                      ? `rounded border px-2 py-1 ${COMPACT_CHIP_CLASS[step.state]}`
                      : `flex min-w-0 items-center gap-2 rounded border px-2 py-1.5 text-sm ${CHIP_CLASS[step.state]}`}
                aria-current={step.state === 'current' ? 'step' : undefined}
                data-state={step.state}
                data-test="{dataTest}-{step.key}"
            >
                {#if !plain}
                    <span
                        class="inline-flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-medium {NUMBER_CLASS[
                            step.state
                        ]}"
                        aria-hidden="true"
                        data-test="{dataTest}-{step.key}-number"
                    >
                        {index + 1}
                    </span>
                {/if}
                <span class={plain ? '' : 'min-w-0 break-words'} data-test="{dataTest}-{step.key}-label"
                    >{words && index > 0 ? '→ ' : ''}{step.label}</span
                >
                {#if step.state === 'done' && !plain}
                    <CheckIcon size="0.875rem" weight="bold" class="shrink-0" aria-hidden="true" />
                {/if}
                <span class="sr-only">{t(STATE_KEY[step.state], lang)}</span>
            </li>
        {/each}
    </ol>
    {#if nowText}
        <p class="mt-2 text-sm text-ink-800" data-test="{dataTest}-now">{nowText}</p>
    {/if}
</div>
