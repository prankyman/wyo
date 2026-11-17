<script lang="ts">
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * One open section holding its own form, the way a card's section is one form of its own: a box the server
     * refused (`refusal`, drawn by `Field`) and the section's own «Сохранить».
     */
    interface IProps {
        lang: TLocale;
        refusal?: string | null;
    }

    let { lang, refusal = null }: IProps = $props();
</script>

<CollapsibleSection {lang} id="client-section-ids" title="Identifiers" open headingLevel="h3">
    <form data-test="the-section-form" onsubmit={(event) => event.preventDefault()}>
        <Field label="Tax number" for="inn" testId="client-inn" error={refusal}>
            <input id="inn" name="inn" data-test="client-inn" />
        </Field>
        <button type="submit" data-test="the-section-submit">Save</button>
    </form>
</CollapsibleSection>
