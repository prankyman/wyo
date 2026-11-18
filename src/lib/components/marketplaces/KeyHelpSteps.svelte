<script lang="ts">
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StepStrip from '$lib/components/ui/StepStrip.svelte';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * How a seller makes the key an adapter asks for, step by step, beside the key form: a folded section with the strip of the
     * steps' short names and, under it, every step in full. The texts belong to the adapter: `marketplace.<code>.keyHelp.intro`
     * and `marketplace.<code>.step.<n>.title` / `.text`, from 1 on, as many as it has. They are written from the marketplace's
     * own instruction, read on the day and recorded in the research file of 2026-11-18 on the token check (`.agents/research/`,
     * section 1, the steps press by press). An adapter with no steps draws nothing.
     */
    interface IProps {
        lang: TLocale;
        /** The adapter's code. */
        code: string;
        /** The section's id and `data-test`: its strip is `<id>-strip`, its list `<id>-list`, its introduction `<id>-intro`. */
        id: string;
        /** Open on arrival: when an account is added; folded when a key is replaced. */
        open?: boolean;
    }

    let { lang, code, id, open = false }: IProps = $props();

    const steps = $derived.by(() => {
        const found: { key: string; title: string; text: string }[] = [];
        for (let n = 1; hasMessage(`marketplace.${code}.step.${n}.title`, lang); n += 1) {
            found.push({
                key: String(n),
                title: t(`marketplace.${code}.step.${n}.title`, lang),
                text: t(`marketplace.${code}.step.${n}.text`, lang)
            });
        }
        return found;
    });
    const introKey = $derived(`marketplace.${code}.keyHelp.intro`);
</script>

{#if steps.length > 0}
    <CollapsibleSection
        {lang}
        {id}
        {open}
        headingLevel="h3"
        title={t('accounts.keyHelp.title', lang)}
        help={t('accounts.keyHelp.help', lang)}
    >
        <Stack>
            {#if hasMessage(introKey, lang)}
                <p class="text-sm text-ink-700" data-test="{id}-intro">{t(introKey, lang)}</p>
            {/if}
            <StepStrip
                {lang}
                dataTest="{id}-strip"
                steps={steps.map((step) => ({ key: step.key, label: step.title, state: 'todo' as const }))}
            />
            <ol class="flex list-none flex-col gap-2 text-sm text-ink-800" data-test="{id}-list">
                {#each steps as step (step.key)}
                    <li data-test="{id}-step-{step.key}">
                        <span class="font-medium">{step.key}. {step.title}.</span>
                        {step.text}
                    </li>
                {/each}
            </ol>
        </Stack>
    </CollapsibleSection>
{/if}
