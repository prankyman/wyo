<script lang="ts">
    import type { Snippet } from 'svelte';
    import { ERROR_CLASS } from './field-classes';

    /**
     * The refusal of a field that draws its own label (a finder whose mode row is its label), so `Field`
     * cannot wrap it: the control, and under it the refusal in the same look `Field` gives one. While a refusal is shown the
     * box carries `data-field-error`, so a closed section around it opens.
     */
    interface IProps {
        /** The refusal, already worded; nothing is drawn while it is empty. */
        error?: string | null;
        /** `data-test` of the refusal; the box itself is `<testId>` with `-box`. */
        testId: string;
        class?: string;
        children: Snippet;
    }

    let { error = null, testId, class: className = '', children }: IProps = $props();
</script>

<div class="flex flex-col gap-1.5 {className}" data-field-error={error ? '' : undefined} data-test="{testId}-box">
    {@render children()}
    {#if error}
        <span role="alert" class={ERROR_CLASS} data-test={testId}>{error}</span>
    {/if}
</div>
