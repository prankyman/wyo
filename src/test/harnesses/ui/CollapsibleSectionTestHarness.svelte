<script lang="ts">
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * Two sections in one form, the way a card spans its sections with one «Сохранить»: the first holds a box the browser
     * itself checks (`required`), the second a box the server refused (`refusal`, drawn by `Field`).
     */
    interface IProps {
        lang: TLocale;
        open?: boolean;
        refusal?: string | null;
        withSummary?: boolean;
    }

    let { lang, open = false, refusal = null, withSummary = false }: IProps = $props();
</script>

<form data-test="the-form" onsubmit={(event) => event.preventDefault()}>
    <CollapsibleSection {lang} id="client-section-ids" title="Identifiers" {open} headingLevel="h3">
        {#snippet summary()}{#if withSummary}3 rows{/if}{/snippet}
        <Field label="Tax number" for="inn" testId="client-inn">
            <input id="inn" name="inn" required data-test="client-inn" />
        </Field>
    </CollapsibleSection>
    <CollapsibleSection {lang} id="client-section-bank" title="Bank">
        <Field label="Account" help="Twenty digits." for="account" testId="client-account" error={refusal}>
            <input id="account" name="account" data-test="client-account" />
        </Field>
    </CollapsibleSection>
    <button type="submit" data-test="the-submit">Save</button>
</form>
