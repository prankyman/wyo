<script lang="ts">
    import { copyToClipboard } from '$lib/utils/clipboard';
    import CheckIcon from 'phosphor-svelte/lib/CheckIcon';
    import CopyIcon from 'phosphor-svelte/lib/CopyIcon';
    import Button from './Button.svelte';
    import IconButton from './IconButton.svelte';

    /**
     * The «Копировать» button with its answer: a press puts `text` on the clipboard and the words beside the
     * button say whether it worked — «Скопировано» in green, or, when the browser refused, the words asking to copy by hand.
     * Every copy button of the website is this component; each place gives its own words.
     */
    interface IProps {
        /** What is copied. */
        text: string;
        /** The button's words, already translated. */
        label: string;
        /** Said after a copy that worked. */
        copiedLabel: string;
        /** Said when the browser refused the clipboard. */
        failedLabel: string;
        variant?: 'primary' | 'secondary';
        disabled?: boolean;
        /** The button is `<dataTest>`, its answer `<dataTest>-state` unless `stateTestId` names it. */
        dataTest: string;
        stateTestId?: string;
        /**
         * `button` — the button with its words and the answer beside it; `icon` — a copy icon named by its
         * tooltip, for a row of a list beside the red delete icon: a tick replaces it after a copy, and the answer is said to a
         * screen reader and shown in the tooltip.
         */
        look?: 'button' | 'icon';
        class?: string;
    }

    let {
        text,
        label,
        copiedLabel,
        failedLabel,
        variant = 'secondary',
        disabled = false,
        dataTest,
        stateTestId,
        look = 'button',
        class: className = ''
    }: IProps = $props();

    let state = $state<'idle' | 'copied' | 'failed'>('idle');

    async function copy(): Promise<void> {
        state = (await copyToClipboard(text)) ? 'copied' : 'failed';
    }
</script>

{#if look === 'icon'}
    <span class="inline-flex {className}">
        <IconButton
            tooltip={state === 'copied' ? copiedLabel : state === 'failed' ? failedLabel : label}
            ariaLabel={label}
            {disabled}
            onclick={copy}
            data-test={dataTest}
        >
            {#snippet icon()}
                {#if state === 'copied'}
                    <CheckIcon size="1.25rem" class="text-ok-700" aria-hidden="true" />
                {:else}
                    <CopyIcon size="1.25rem" aria-hidden="true" />
                {/if}
            {/snippet}
        </IconButton>
        <span class="sr-only" role="status" data-test={stateTestId ?? `${dataTest}-state`}>
            {state === 'copied' ? copiedLabel : state === 'failed' ? failedLabel : ''}
        </span>
    </span>
{:else}
    <span class="inline-flex flex-wrap items-center gap-2 {className}">
        <Button {variant} {disabled} onclick={copy} {dataTest}>{label}</Button>
        <span class="text-xs" role="status" data-test={stateTestId ?? `${dataTest}-state`}>
            {#if state === 'copied'}
                <span class="text-ok-800">{copiedLabel}</span>
            {:else if state === 'failed'}
                <span class="text-warn-800">{failedLabel}</span>
            {/if}
        </span>
    </span>
{/if}
