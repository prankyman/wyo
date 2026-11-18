<script lang="ts">
    import Button from '$lib/components/ui/Button.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
    import { toneClass, type TTone } from '$lib/components/ui/status-tones';

    /**
     * One card of «Обзор» that says how one part of the installation stands: a title with its «?», a state in the colour of its
     * tone, one sentence that says what it means and what to do, and, when there is something to press, one action. The version,
     * the certificate and the last copy of the database are drawn by it, and so is the card a later plan adds.
     */
    interface IProps {
        /** The card's `data-test`; its parts are `<testId>-help`, `<testId>-state`, `<testId>-sentence` and `<testId>-action`. */
        testId: string;
        title: string;
        /** The text behind the title's «?»: what the card is, why it is there, what the person does about it. */
        help: string;
        state: { label: string; tone: TTone };
        sentence: string;
        action?: { label: string; href: string };
    }

    let { testId, title, help, state, sentence, action }: IProps = $props();
</script>

<section class="rounded border border-ink-200 bg-white p-4" data-test={testId}>
    <Stack>
        <HeadingWithHelp text={title} {help} level="h3" look="section" testId="{testId}-help" />
        <div>
            <StatusBadge label={state.label} colorClass={toneClass(state.tone)} size="md" dataTest="{testId}-state" />
        </div>
        <p class="text-sm text-ink-700" data-test="{testId}-sentence">{sentence}</p>
        {#if action}
            <div>
                <Button href={action.href} variant="secondary" dataTest="{testId}-action">{action.label}</Button>
            </div>
        {/if}
    </Stack>
</section>
