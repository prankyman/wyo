<script lang="ts">
    import { refusalKey } from '$lib/api/refusal-key';
    import Button from '$lib/components/ui/Button.svelte';
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * The place of a section of «Внешний API» while its data has not come: the section's folded title with its «?», and inside it
     * «Читаем…» while the wyo server answers, or the refusal's words with «Повторить» when it refused. The page draws the section
     * itself (a form built from the data) once the data is there.
     */
    interface IProps {
        lang: TLocale;
        /** The section's id: its `data-test`; `<id>-loading`, `<id>-refusal` and `<id>-retry` are its parts. */
        id: string;
        title: string;
        help: string;
        /** The refusal's code; null while the section waits. */
        refusal: string | null;
        onRetry: () => void;
    }

    let { lang, id, title, help, refusal, onRetry }: IProps = $props();
</script>

<CollapsibleSection {lang} {id} {title} {help}>
    <Stack>
        {#if refusal}
            <NoteBox tone="error" role="alert" dataTest="{id}-refusal">{t(refusalKey(refusal), lang)}</NoteBox>
            <div>
                <Button variant="secondary" dataTest="{id}-retry" onclick={onRetry}>{t('common.retry', lang)}</Button>
            </div>
        {:else}
            <p class="text-ink-600" data-test="{id}-loading">{t('externalApi.section.loading', lang)}</p>
        {/if}
    </Stack>
</CollapsibleSection>
