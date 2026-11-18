<script lang="ts">
    import StepStrip from '$lib/components/ui/StepStrip.svelte';
    import type { IStepStripStep, TStepStripState } from '$lib/components/ui/step-strip';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { THandoverStepState } from './types';

    /**
     * The steps of a hand-over, exactly the ones it is given — the steps the marketplace's adapter declares, in the order they
     * run (a new batch, moving the orders, adding boxes, the shipping point, closing, the documents: six for one marketplace,
     * four for another) — as a `StepStrip`, with «Сейчас: …» naming the step, out of how many, that is running, waits for the
     * owner or failed. A step that waits or failed
     * says so in its own words too, so its state is never only a colour.
     */
    interface IProps {
        lang: TLocale;
        steps: THandoverStepState[];
        dataTest: string;
    }

    let { lang, steps, dataTest }: IProps = $props();

    const STRIP_STATE: Readonly<Record<THandoverStepState['state'], TStepStripState>> = {
        done: 'done',
        now: 'current',
        waiting: 'current',
        failed: 'current',
        todo: 'todo'
    };

    const strip = $derived<IStepStripStep[]>(
        steps.map((one) => {
            const name = t(`handovers.step.${one.step}`, lang);
            const note =
                one.state === 'waiting' || one.state === 'failed'
                    ? ` — ${t(`handovers.stepNote.${one.state}`, lang)}`
                    : '';
            return { key: one.step, label: `${name}${note}`, state: STRIP_STATE[one.state] };
        })
    );

    const nowText = $derived.by(() => {
        const at = steps.findIndex((one) => STRIP_STATE[one.state] === 'current');
        if (at === -1) {
            return t(
                steps.length > 0 && steps.every((one) => one.state === 'done')
                    ? 'handovers.now.done'
                    : 'handovers.now.notBegun',
                lang
            );
        }
        return t(`handovers.now.${steps[at].state}`, lang, {
            number: at + 1,
            total: steps.length,
            step: t(`handovers.step.${steps[at].step}`, lang).toLowerCase()
        });
    });
</script>

<StepStrip {lang} steps={strip} {nowText} {dataTest} />
