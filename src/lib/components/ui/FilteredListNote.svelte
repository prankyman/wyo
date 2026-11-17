<script lang="ts">
    import Button from './Button.svelte';

    /**
     * The line above a list that says it is filtered, with the press that shows everything again: «Показаны записи со статусом …»
     * with «Показать все записи». One piece for every list that a link or a search narrows; the words come from the
     * caller, already translated.
     */
    interface IProps {
        /** What is shown, already worded: why the list is shorter than usual. */
        text: string;
        /** The press that lists everything. */
        clearLabel: string;
        /** Where it goes, already localized. */
        clearHref: string;
        dataTest: string;
        clearDataTest: string;
    }

    let { text, clearLabel, clearHref, dataTest, clearDataTest }: IProps = $props();
</script>

<p class="flex flex-wrap items-center gap-3 text-sm" data-test={dataTest}>
    <span class="text-ink-700">{text}</span>
    <Button href={clearHref} variant="secondary" dataTest={clearDataTest}>{clearLabel}</Button>
</p>
