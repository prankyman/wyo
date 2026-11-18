<script lang="ts">
    import { api } from '$lib/api/api';
    import type { components } from '$lib/api/wyo-api';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { formatBytes } from '$lib/i18n/format-bytes';
    import { formatDate } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * «Резервные копии» of «Настройки»: the last seven copies of the database with their day and size, newest first
     * (`GET /api/v1/backups`), «Сделать копию сейчас» (`POST /api/v1/backups`) and the sentence about the key file. It holds no
     * setting, so it is a folded section of its own and not a `SettingsSection`. The page keeps the list and is told a new copy
     * (`onCreated`).
     */
    interface IProps {
        lang: TLocale;
        backups: components['schemas']['Backup'][];
        onCreated: (backup: components['schemas']['Backup']) => void;
    }

    let { lang, backups, onCreated }: IProps = $props();

    const create = new ApiAction();

    async function makeCopy(): Promise<void> {
        const result = await create.run(
            () => api.post<components['schemas']['Backup']>('/api/v1/backups', {}),
            'settings.backups.created'
        );
        if (result?.ok) onCreated(result.data);
    }
</script>

<CollapsibleSection
    {lang}
    id="settings-backups"
    title={t('settings.backups.title', lang)}
    help={t('settings.backups.help', lang)}
>
    <Stack>
        <NoteBox tone="warning" dataTest="backups-key-file">{t('settings.backups.keyFile', lang)}</NoteBox>
        {#if backups.length === 0}
            <EmptyState message={t('settings.backups.empty', lang)} dataTest="backups-empty" />
        {:else}
            <ul class="flex flex-col gap-1 text-sm text-ink-700">
                {#each backups as backup (backup.name)}
                    <li data-test="backup-row">
                        {backup.name} — {formatDate(backup.createdAt, lang)}, {formatBytes(backup.size, lang)}
                    </li>
                {/each}
            </ul>
        {/if}
        <div>
            <ButtonWithHelp
                variant="secondary"
                pending={create.pending}
                dataTest="backups-create"
                help={t('settings.backups.createHelp', lang)}
                helpTestId="backups-create-help"
                helpLabel={t('settings.backups.create', lang)}
                onclick={makeCopy}
            >
                {t('settings.backups.create', lang)}
            </ButtonWithHelp>
            <FormAnswer saved={create.saved} error={create.error} dataTest="backups-create-answer" />
        </div>
    </Stack>
</CollapsibleSection>
