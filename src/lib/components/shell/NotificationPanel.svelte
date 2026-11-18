<script lang="ts">
    import { FOCUS_RING, PRESS_FEEDBACK } from '$lib/components/ui/press';
    import { localizePath, t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import type { INotificationLine } from '$lib/notifications/types';
    import WarningIcon from 'phosphor-svelte/lib/WarningIcon';
    import { URGENCY_STYLE } from './panel-rules';

    /**
     * The inside of the bell's panel, drawn by `NotificationBell` under the bell on a computer and in a full-screen sheet on a
     * phone: «Требует внимания» — the lines, each one link (the whole row, at least 44 px tall) that closes the panel and opens
     * its page. An empty panel teaches what the bell will do.
     */
    interface IProps {
        lang: TLocale;
        lines: INotificationLine[];
        onNavigate: () => void;
    }

    let { lang, lines, onNavigate }: IProps = $props();

    const rowClass = `flex min-h-11 w-full items-start gap-2 rounded px-2 py-2 text-left text-sm ${PRESS_FEEDBACK} ${FOCUS_RING}`;
</script>

<div class="flex flex-col gap-4 p-4" data-test="notification-panel-body">
    {#if lines.length === 0}
        <p class="text-sm text-ink-600" data-test="notification-panel-empty">{t('shell.notifications.empty', lang)}</p>
    {:else}
        <section class="flex flex-col gap-1" data-test="notification-lines">
            <h3 class="text-xs font-semibold text-ink-500">{t('shell.notifications.lines', lang)}</h3>
            <ul class="flex flex-col gap-1">
                {#each lines as line (line.key)}
                    {@const style = URGENCY_STYLE[line.urgency]}
                    <li
                        class="rounded border-l-4 {style.border} {style.background}"
                        data-test="notification-line"
                        data-kind={line.kind}
                        data-urgency={line.urgency}
                    >
                        {#snippet body()}
                            {#if line.urgency === 'urgent'}
                                <WarningIcon class="mt-0.5 size-4 shrink-0 text-stop-700" aria-hidden="true" />
                                <span class="sr-only">{t('shell.notifications.urgent', lang)}:</span>
                            {/if}
                            <span class="text-ink-900" data-test="notification-line-text">{line.text}</span>
                        {/snippet}
                        {#if line.href}
                            <a
                                href={localizePath(line.href, lang)}
                                class="{rowClass} pressable:hover:bg-white/60"
                                data-test="notification-line-link"
                                onclick={onNavigate}
                            >
                                {@render body()}
                            </a>
                        {:else}
                            <div class={rowClass}>{@render body()}</div>
                        {/if}
                    </li>
                {/each}
            </ul>
        </section>
    {/if}
</div>
