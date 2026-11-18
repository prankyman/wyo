<script lang="ts">
    import { api } from '$lib/api/api';
    import { refusalKey } from '$lib/api/refusal-key';
    import type { components } from '$lib/api/wyo-api';
    import BackupsSection from '$lib/components/settings/BackupsSection.svelte';
    import { settingsSections, type TSettingsSection } from '$lib/components/settings/sections';
    import SettingsSection from '$lib/components/settings/SettingsSection.svelte';
    import UpdatesStatus from '$lib/components/settings/UpdatesStatus.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import type { TInstallation } from '$lib/overview/installation-cards';
    import type { TSettingsGroup } from '$lib/validation/settings-group';
    import { onMount } from 'svelte';

    type TBackup = components['schemas']['Backup'];

    /**
     * «Настройки»: folded sections, each one a form with its own «Сохранить». «Обновления» and «Вход и безопасность» are groups of
     * the settings registry (`GET /api/v1/settings`), drawn by `SettingsSection`; «Резервные копии» holds no setting and is
     * `BackupsSection`. The page reads the three things it shows — the registry, the installation (for the version and the
     * updater's request) and the list of copies — and hands each section its part.
     */
    const lang = getLocale();
    /** The seven copies the wyo server keeps. */
    const COPIES_KEPT = 7;

    let sections = $state<TSettingsSection[] | null>(null);
    let installation = $state<TInstallation | null>(null);
    let backups = $state<TBackup[]>([]);
    let refusal = $state<string | null>(null);

    async function read(): Promise<void> {
        refusal = null;
        const [settings, state, copies] = await Promise.all([
            api.get<{ groups: TSettingsGroup[] }>('/api/v1/settings'),
            api.get<TInstallation>('/api/v1/installation'),
            api.get<{ backups: TBackup[] }>('/api/v1/backups')
        ]);
        const failed = [settings, state, copies].find((result) => !result.ok);
        if (failed && !failed.ok) {
            refusal = t(refusalKey(failed.refusal.code), lang);
            return;
        }
        if (settings.ok && state.ok && copies.ok) {
            sections = settingsSections(settings.data.groups);
            installation = state.data;
            backups = copies.data.backups;
        }
    }

    /** The updater was asked for something: the installation says so (`updates.requested`) until it has answered. */
    async function readInstallation(): Promise<void> {
        const state = await api.get<TInstallation>('/api/v1/installation');
        if (state.ok) installation = state.data;
    }

    onMount(() => {
        void read();
    });
</script>

<Stack as="section" dataTest="settings">
    <PageHeading
        title={t('settings.title', lang)}
        help={t('settings.help', lang)}
        headingTestId="settings-heading"
        helpTestId="settings-help"
    />
    {#if sections && installation}
        {@const current = installation}
        <Stack dataTest="settings-sections">
            {#each sections as section (section.kind === 'group' ? section.group.id : section.kind)}
                {#if section.kind === 'backups'}
                    <BackupsSection
                        {lang}
                        {backups}
                        onCreated={(backup) => (backups = [backup, ...backups].slice(0, COPIES_KEPT))}
                    />
                {:else if section.group.id === 'updates'}
                    <SettingsSection {lang} group={section.group}>
                        {#snippet extra()}
                            <UpdatesStatus
                                {lang}
                                version={current.version}
                                updates={current.updates}
                                onRequested={readInstallation}
                            />
                        {/snippet}
                    </SettingsSection>
                {:else}
                    <SettingsSection {lang} group={section.group} />
                {/if}
            {/each}
        </Stack>
    {:else if refusal}
        <NoteBox tone="error" role="alert" dataTest="settings-refusal">{refusal}</NoteBox>
        <div>
            <Button variant="secondary" dataTest="settings-retry" onclick={read}>{t('common.retry', lang)}</Button>
        </div>
    {:else}
        <p class="text-ink-600" data-test="settings-loading">{t('settings.loading', lang)}</p>
    {/if}
</Stack>
