<script lang="ts">
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import { problemHelp, problemSentence, wordsOwner } from './problem-text';
    import type { TProblem } from './types';

    /**
     * A problem of an order or a hand-over: wyo's own sentence for its code (`problem.<code>`, with the step or the identifier it
     * names written out) with its «?» that says what to do, and beside it the marketplace's or the fulfilment centre's own words
     * and code as received. wyo's sentence says what happened in the same words for every marketplace; the other party's words
     * say why, in the words only that party knows. A refusal of the marketplace that its own family of texts words (`marketplace.<code>.refusal.<refusal code>`: a label size it does not make) is worded by it, with the numbers of the refusal written out. A code wyo has no text for is worded by the general text that names the code.
     */
    interface IProps {
        lang: TLocale;
        problem: TProblem;
        /** The code of the marketplace of the order or the hand-over: a refusal it has words of its own for is worded by them. */
        marketplace?: string;
        /** The line is `<testId>`, its «?» `<testId>-help`, the other party's words `<testId>-words`. */
        testId: string;
    }

    let { lang, problem, marketplace, testId }: IProps = $props();

    const sentence = $derived(problemSentence(problem, lang, marketplace));
    const help = $derived(problemHelp(problem, lang, marketplace));
    const whose = $derived(wordsOwner(problem));
</script>

<NoteBox tone="error" role="note" dataTest={testId}>
    <Stack>
        <p class="flex flex-wrap items-center gap-1">
            <span>{sentence}</span>
            <HelpTooltip tooltip={help} ariaLabel={sentence} testId="{testId}-help" />
        </p>
        {#if problem.words}
            <p class="break-words" data-test="{testId}-words">
                {t(`problem.words.${whose}`, lang)}: «{problem.words}»{problem.marketplaceCode &&
                problem.marketplaceCode !== problem.words
                    ? ` (${t('problem.code', lang, { code: problem.marketplaceCode })})`
                    : ''}
            </p>
        {/if}
    </Stack>
</NoteBox>
