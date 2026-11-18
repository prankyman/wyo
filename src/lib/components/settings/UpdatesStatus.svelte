<script lang="ts">
    import { api } from '$lib/api/api';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { ApiAction } from '$lib/forms/api-action.svelte';
    import { formatDate } from '$lib/i18n/format-date';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TInstallation } from '$lib/overview/installation-cards';

    /**
     * What «Обновления» of «Настройки» tells besides the switch: the installed version, the last look for a newer one, the
     * version that waits, and the two requests the owner can make — «Проверить сейчас» and «Обновить сейчас». The wyo server only
     * writes the request down; the separate updater program does it within a minute, so both buttons are blocked until it has
     * answered, and the page is told (`onRequested`) so it reads the installation again.
     */
    interface IProps {
        lang: TLocale;
        version: string;
        updates: TInstallation['updates'];
        onRequested: () => void;
    }

    let { lang, version, updates, onRequested }: IProps = $props();

    const check = new ApiAction();
    const install = new ApiAction();

    async function request(action: ApiAction, path: string, savedKey: string): Promise<void> {
        const result = await action.run(() => api.post(path, {}), savedKey);
        if (result?.ok) onRequested();
    }
</script>

<Stack>
    <div class="flex flex-col gap-1 text-sm text-ink-700">
        <p data-test="updates-version">{t('settings.updates.version', lang, { version })}</p>
        <p data-test="updates-last-look">
            {updates.lastCheckAt
                ? t('settings.updates.lastLook', lang, { date: formatDate(updates.lastCheckAt, lang) })
                : t('settings.updates.neverLooked', lang)}
        </p>
        {#if updates.waitingVersion}
            <p data-test="updates-waiting">
                {t('settings.updates.waiting', lang, { version: updates.waitingVersion })}
            </p>
        {/if}
    </div>
    {#if updates.requested}
        <NoteBox tone="info" role="status" dataTest="updates-requested">
            {t('settings.updates.requested', lang)}
        </NoteBox>
    {/if}
    <div class="flex flex-col gap-4 sm:flex-row sm:flex-wrap">
        <div>
            <ButtonWithHelp
                variant="secondary"
                disabled={updates.requested}
                pending={check.pending}
                dataTest="updates-check"
                help={t('settings.updates.checkHelp', lang)}
                helpTestId="updates-check-help"
                helpLabel={t('settings.updates.check', lang)}
                onclick={() => request(check, '/api/v1/updates/check', 'settings.updates.checkRequested')}
            >
                {t('settings.updates.check', lang)}
            </ButtonWithHelp>
            <FormAnswer saved={check.saved} error={check.error} dataTest="updates-check-answer" />
        </div>
        <div>
            <ButtonWithHelp
                variant="secondary"
                disabled={updates.requested}
                pending={install.pending}
                dataTest="updates-install"
                help={t('settings.updates.installHelp', lang)}
                helpTestId="updates-install-help"
                helpLabel={t('settings.updates.install', lang)}
                onclick={() => request(install, '/api/v1/updates/install', 'settings.updates.installRequested')}
            >
                {t('settings.updates.install', lang)}
            </ButtonWithHelp>
            <FormAnswer saved={install.saved} error={install.error} dataTest="updates-install-answer" />
        </div>
    </div>
</Stack>
