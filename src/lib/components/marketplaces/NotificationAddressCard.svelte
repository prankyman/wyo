<script lang="ts">
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import CopyButton from '$lib/components/ui/CopyButton.svelte';
    import FactList from '$lib/components/ui/FactList.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import { COPY_BOX_CLASS } from '$lib/components/ui/field-classes';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import type { TTone } from '$lib/components/ui/status-tones';
    import { formatDateTime } from '$lib/i18n/format-date';
    import { hasMessage, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { TAccountNotifications } from './types';

    /**
     * An account's notification address, for a marketplace that sends the wyo server a notification when an order appears or
     * changes: the whole address to copy (the wyo server's own origin and the account's path), whether the notifications arrive
     * («не подключены», «подключены», «не доходят»; `accounts.notifications.<state>`), the time of the last one, and, when they
     * stopped arriving, a note that says what to check and how often wyo reads meanwhile. Under them the steps that connect the
     * notifications in the marketplace's cabinet, written by the marketplace's family of texts
     * (`marketplace.<code>.notifications.step.<n>.title` / `.text` from 1 on, its introduction `.intro`, what to check when
     * they are lost `.lost.check`); a marketplace with no such texts draws no steps. The steps arrive open while the
     * notifications are not connected yet and folded once they are.
     */
    interface IProps {
        lang: TLocale;
        /** The adapter's code, which names the family of the steps. */
        code: string;
        notifications: TAccountNotifications;
        /** The wyo server's own address as the owner reaches it; the page's own origin when left out. */
        origin?: string;
        /** The block is `<testId>`, its address `<testId>-address`, its copy button `<testId>-copy`, its steps `<testId>-steps`. */
        testId?: string;
    }

    let { lang, code, notifications, origin = window.location.origin, testId = 'notifications' }: IProps = $props();

    const TONE: Readonly<Record<TAccountNotifications['state'], TTone>> = {
        waiting: 'wait',
        working: 'ok',
        lost: 'warn'
    };

    const address = $derived(`${origin}${notifications.path}`);
    const lostCheckKey = $derived(`marketplace.${code}.notifications.lost.check`);
    const introKey = $derived(`marketplace.${code}.notifications.intro`);

    const steps = $derived.by(() => {
        const found: { key: string; title: string; text: string }[] = [];
        for (let n = 1; hasMessage(`marketplace.${code}.notifications.step.${n}.title`, lang); n += 1) {
            found.push({
                key: String(n),
                title: t(`marketplace.${code}.notifications.step.${n}.title`, lang),
                text: t(`marketplace.${code}.notifications.step.${n}.text`, lang)
            });
        }
        return found;
    });

    const facts = $derived(
        notifications.seenAt
            ? [
                  {
                      key: 'seenAt',
                      label: t('accounts.notifications.seenAt', lang),
                      value: formatDateTime(notifications.seenAt, lang),
                      help: t('accounts.notifications.seenAt.help', lang)
                  }
              ]
            : []
    );
</script>

<Stack dataTest={testId}>
    <div class="flex flex-wrap items-center justify-between gap-2">
        <HeadingWithHelp
            text={t('accounts.notifications.title', lang)}
            help={t('accounts.notifications.title.help', lang)}
            level="h3"
            look="section"
            testId="{testId}-title-help"
            headingTestId="{testId}-heading"
        />
        <StatusBadgeWithHelp
            label={t(`accounts.notifications.${notifications.state}`, lang)}
            tone={TONE[notifications.state]}
            help={t(`accounts.notifications.${notifications.state}.help`, lang)}
            size="md"
            testId="{testId}-state"
        />
    </div>
    {#if notifications.state === 'lost'}
        <NoteBox tone="warning" dataTest="{testId}-lost">
            {t('accounts.notifications.lost.note', lang, {
                date: notifications.lostAt ? formatDateTime(notifications.lostAt, lang) : ''
            })}
            {hasMessage(lostCheckKey, lang) ? t(lostCheckKey, lang) : ''}
        </NoteBox>
    {/if}
    <Field
        label={t('accounts.notifications.address', lang)}
        help={t('accounts.notifications.address.help', lang)}
        testId="{testId}-address-field"
    >
        <Stack>
            <code class={COPY_BOX_CLASS} data-test="{testId}-address">{address}</code>
            <div>
                <CopyButton
                    text={address}
                    label={t('app.ui.showOnce.copy', lang)}
                    copiedLabel={t('app.ui.showOnce.copied', lang)}
                    failedLabel={t('app.ui.showOnce.copyFailed', lang)}
                    dataTest="{testId}-copy"
                />
            </div>
        </Stack>
    </Field>
    {#if facts.length > 0}
        <FactList items={facts} dataTest="{testId}-facts" />
    {/if}
    {#if steps.length > 0}
        <CollapsibleSection
            {lang}
            id="{testId}-steps"
            open={notifications.state === 'waiting'}
            headingLevel="h4"
            title={t('accounts.notifications.steps.title', lang)}
            help={t('accounts.notifications.steps.help', lang)}
        >
            <Stack>
                {#if hasMessage(introKey, lang)}
                    <p class="text-sm text-ink-700" data-test="{testId}-steps-intro">{t(introKey, lang)}</p>
                {/if}
                <ol class="flex list-none flex-col gap-2 text-sm text-ink-800" data-test="{testId}-steps-list">
                    {#each steps as step (step.key)}
                        <li data-test="{testId}-steps-step-{step.key}">
                            <span class="font-medium">{step.key}. {step.title}.</span>
                            {step.text}
                        </li>
                    {/each}
                </ol>
            </Stack>
        </CollapsibleSection>
    {/if}
</Stack>
