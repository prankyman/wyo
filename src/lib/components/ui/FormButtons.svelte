<script lang="ts">
    import type { Snippet } from 'svelte';

    interface IProps {
        /** The buttons of the form: its main button first, then «Закрыть» (`DisclosureClose`), a «?», a link back. */
        children: Snippet;
        /**
         * What is drawn on a line of its own under the buttons: the form's answer (`FormAnswer`), a short hint. Named
         * `below`, not `answer`, because many forms already hold a value called `answer`, which a snippet would shadow.
         */
        below?: Snippet;
        /** Classes of the row's box (a margin above it). */
        class?: string;
        dataTest?: string;
    }

    let { children, below, class: className = '', dataTest }: IProps = $props();
</script>

<!--
    The buttons of one form: a row of their own under all the fields, at
    the left edge, at every width. In a form laid out as a grid the row spans every column (`col-span-full`), so it never
    becomes one more cell among the fields — the cell where a button would land depends on the number of fields and on the
    width of the screen, which is how a filter's «Показать» once floated at the right of the third row.
    `src/lib/components/ui/form-buttons-policy.test.ts` fails a submit button drawn as a cell of a grid form.
-->
<div class="col-span-full {className}" data-form-buttons data-test={dataTest}>
    <div class="flex flex-wrap items-center gap-2">
        {@render children()}
    </div>
    {#if below}
        {@render below()}
    {/if}
</div>
