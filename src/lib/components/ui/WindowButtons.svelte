<script lang="ts">
    import type { Snippet } from 'svelte';

    interface IProps {
        /** The buttons: «Отмена» or «Закрыть» first, the window's own action last. */
        children: Snippet;
        /** A figure for the press at the left of the row (the label total of the delivery card's «Печать этикеток»). */
        note?: Snippet;
        /**
         * The answer of the press on a line above the row (a refusal), so it is seen where the button is pressed. Drawn as it
         * is, with no box of its own: an `{#if}` inside that draws nothing leaves no gap above the buttons.
         */
        above?: Snippet;
        dataTest?: string;
    }

    let { children, note, above, dataTest }: IProps = $props();
</script>

<!--
    The buttons of a window (constitution, "Windows"): drawn as the
    `footer` of `ModalWindow`, the band that stands still under the content, so the buttons are seen however long the content is.
    One row at the right, the cancel first and the action last — the same in every window. The buttons are one group: on a narrow
    phone the group wraps under the note as a whole, so «Сохранить» never drops under «Закрыть».
    `window-buttons-policy.test.ts` fails a window that draws a button anywhere else.
-->
<div class="flex flex-col gap-2" data-window-buttons data-test={dataTest}>
    {@render above?.()}
    <div class="flex flex-wrap items-center justify-end gap-3">
        {#if note}
            <div class="mr-auto">{@render note()}</div>
        {/if}
        <div class="flex flex-wrap items-center justify-end gap-3">
            {@render children()}
        </div>
    </div>
</div>
