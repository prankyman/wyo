<script lang="ts">
    import { cn } from '$lib/utils/cn';
    import { CLICKABLE_ROW } from './press';
    import type { IClickableRowProps } from './types';

    /**
     * A table row that opens its record wherever it is pressed — the desktop counterpart of
     * `ClickableCard`. Put a `StretchedLink` in exactly one cell (the record's number or name); its
     * hit area is stretched over the row. Other controls in the row keep working on their own.
     *
     * The row is the containing block of the stretched area (`position: relative` on a table row).
     * Checked in a real browser in Chromium and in WebKit (Playwright's),
     * both as a table row and as a block-level card row on a phone.
     */
    let { dataTest, class: className, children, ...rest }: IClickableRowProps = $props();
</script>

<tr class={cn(CLICKABLE_ROW, className)} data-test={dataTest} {...rest}>
    {@render children()}
</tr>
