<script lang="ts">
    import ArchiveIcon from 'phosphor-svelte/lib/ArchiveIcon';
    import CheckIcon from 'phosphor-svelte/lib/CheckIcon';
    import PencilSimpleIcon from 'phosphor-svelte/lib/PencilSimpleIcon';
    import PlusIcon from 'phosphor-svelte/lib/PlusIcon';
    import TrashIcon from 'phosphor-svelte/lib/TrashIcon';
    import AttentionIcon from '$lib/components/ui/AttentionIcon.svelte';
    import BackLink from '$lib/components/ui/BackLink.svelte';
    import BlockedButton from '$lib/components/ui/BlockedButton.svelte';
    import Button from '$lib/components/ui/Button.svelte';
    import ButtonWithHelp from '$lib/components/ui/ButtonWithHelp.svelte';
    import ClickableCard from '$lib/components/ui/ClickableCard.svelte';
    import ClickableRow from '$lib/components/ui/ClickableRow.svelte';
    import CopyButton from '$lib/components/ui/CopyButton.svelte';
    import DisclosureWithHelp from '$lib/components/ui/DisclosureWithHelp.svelte';
    import EmptyState from '$lib/components/ui/EmptyState.svelte';
    import FilteredListNote from '$lib/components/ui/FilteredListNote.svelte';
    import HeadingWithHelp from '$lib/components/ui/HeadingWithHelp.svelte';
    import HelpLinkList from '$lib/components/ui/HelpLinkList.svelte';
    import HelpTooltip from '$lib/components/ui/HelpTooltip.svelte';
    import IconButton from '$lib/components/ui/IconButton.svelte';
    import IconTooltip from '$lib/components/ui/IconTooltip.svelte';
    import NoteBox from '$lib/components/ui/NoteBox.svelte';
    import PageHeading from '$lib/components/ui/PageHeading.svelte';
    import Pagination from '$lib/components/ui/Pagination.svelte';
    import { pageChange } from '$lib/components/ui/page-change.svelte';
    import PrevNextLinks from '$lib/components/ui/PrevNextLinks.svelte';
    import RemovableChip from '$lib/components/ui/RemovableChip.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import StatusBadge from '$lib/components/ui/StatusBadge.svelte';
    import StatusBadgeWithHelp from '$lib/components/ui/StatusBadgeWithHelp.svelte';
    import { toneClass, type TTone } from '$lib/components/ui/status-tones';
    import StepStrip from '$lib/components/ui/StepStrip.svelte';
    import type { IStepStripStep } from '$lib/components/ui/step-strip';
    import StretchedLink from '$lib/components/ui/StretchedLink.svelte';
    import Tabs from '$lib/components/ui/Tabs.svelte';
    import Toaster from '$lib/components/ui/toast/Toaster.svelte';
    import type { TButtonVariantEnum } from '$lib/components/ui/types';
    import WarningBanner from '$lib/components/ui/WarningBanner.svelte';
    import { getLocale, t } from '$lib/i18n/i18n';
    import { toast } from '$lib/stores/toast.svelte';

    /**
     * The showcase of the basic components: every one drawn in each of its states, so the theme can be judged by eye and the
     * browser tests can measure the page at 390 pixels. Each component is tested once in its own file; this section only
     * draws them. It carries its own `Toaster`, so the toast buttons show their answer on this page; a layout that mounts
     * the `Toaster` itself makes that one redundant.
     */
    const lang = getLocale();

    const VARIANTS: { variant: TButtonVariantEnum; label: string }[] = [
        { variant: 'primary', label: t('showcase.basics.variant.primary') },
        { variant: 'secondary', label: t('showcase.basics.variant.secondary') },
        { variant: 'danger', label: t('showcase.basics.variant.danger') },
        { variant: 'danger-outline', label: t('showcase.basics.variant.dangerOutline') },
        { variant: 'success', label: t('showcase.basics.variant.success') },
        { variant: 'warning', label: t('showcase.basics.variant.warning') },
        { variant: 'ghost', label: t('showcase.basics.variant.ghost') },
        { variant: 'link', label: t('showcase.basics.variant.link') }
    ];

    const TONES: { tone: TTone; label: string }[] = [
        { tone: 'ok', label: t('showcase.basics.tone.ok') },
        { tone: 'wait', label: t('showcase.basics.tone.wait') },
        { tone: 'warn', label: t('showcase.basics.tone.warn') },
        { tone: 'stop', label: t('showcase.basics.tone.stop') },
        { tone: 'info', label: t('showcase.basics.tone.info') },
        { tone: 'off', label: t('showcase.basics.tone.off') }
    ];

    const HELP_LINKS = [
        { label: t('showcase.basics.link.first'), href: 'https://example.org/first-article' },
        { label: t('showcase.basics.link.second'), href: 'https://example.org/second-article' }
    ];

    const STEPS: IStepStripStep[] = [
        { key: 'details', label: t('showcase.basics.step.details'), state: 'done' },
        { key: 'items', label: t('showcase.basics.step.items'), state: 'current' },
        { key: 'review', label: t('showcase.basics.step.review'), state: 'todo' },
        { key: 'send', label: t('showcase.basics.step.send'), state: 'todo' }
    ];

    let activeTab = $state('general');
    const tabs = [
        { id: 'general', label: t('showcase.basics.tab.general') },
        { id: 'keys', label: t('showcase.basics.tab.keys'), count: 3 },
        { id: 'log', label: t('showcase.basics.tab.log'), help: t('showcase.basics.tab.logHelp') }
    ];

    let page = $state(1);
    let listPage = $state(2);
    let chipRemoved = $state(false);

    /** Shows the bar of a page change for a moment, as a plain form answer loading would. */
    function showPageChangeBar(): void {
        pageChange.nativeSubmission = true;
        setTimeout(() => (pageChange.nativeSubmission = false), 1500);
    }
</script>

<Stack as="section" dataTest="showcase-basics">
    <Stack as="section" dataTest="showcase-basics-buttons">
        <HeadingWithHelp text={t('showcase.basics.buttons.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-button-variants">
            {#each VARIANTS as { variant, label } (variant)}
                <Button {variant} dataTest="showcase-basics-button-{variant}">{label}</Button>
            {/each}
            <span class="inline-flex rounded bg-brand-600 p-1">
                <Button variant="ghost-inverse" dataTest="showcase-basics-button-ghost-inverse">
                    {t('showcase.basics.variant.ghostInverse')}
                </Button>
            </span>
        </div>
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-button-sizes">
            <Button size="md" dataTest="showcase-basics-button-md">{t('showcase.basics.size.md')}</Button>
            <Button size="lg" dataTest="showcase-basics-button-lg">{t('showcase.basics.size.lg')}</Button>
            <Button size="xl" dataTest="showcase-basics-button-xl">{t('showcase.basics.size.xl')}</Button>
        </div>
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-button-states">
            <Button disabled dataTest="showcase-basics-button-disabled">{t('showcase.basics.state.disabled')}</Button>
            <Button pending dataTest="showcase-basics-button-pending">{t('showcase.basics.state.pending')}</Button>
            <Button href="/showcase" dataTest="showcase-basics-button-as-link">
                {t('showcase.basics.state.link')}
            </Button>
            <Button dataTest="showcase-basics-button-icon">
                {#snippet icon()}<PlusIcon aria-hidden="true" />{/snippet}
                {t('showcase.basics.state.withIcon')}
            </Button>
            <Button dataTest="showcase-basics-button-icon-only" ariaLabel={t('showcase.basics.state.iconOnly')}>
                {#snippet icon()}<PlusIcon aria-hidden="true" />{/snippet}
            </Button>
        </div>
        <Button fullWidth dataTest="showcase-basics-button-full">{t('showcase.basics.state.fullWidth')}</Button>
        <ButtonWithHelp
            help={t('showcase.basics.withHelp.help')}
            helpTestId="showcase-basics-button-with-help-help"
            helpLabel={t('showcase.basics.withHelp.button')}
            dataTest="showcase-basics-button-with-help"
        >
            {t('showcase.basics.withHelp.button')}
        </ButtonWithHelp>
        <BlockedButton
            reason={t('showcase.basics.blocked.reason')}
            reasonTestId="showcase-basics-blocked-reason"
            help={t('showcase.basics.blocked.help')}
            helpTestId="showcase-basics-blocked-help"
            helpLabel={t('showcase.basics.blocked.button')}
            dataTest="showcase-basics-blocked"
        >
            {t('showcase.basics.blocked.button')}
        </BlockedButton>
        <BlockedButton reason={null} dataTest="showcase-basics-unblocked">
            {t('showcase.basics.blocked.free')}
        </BlockedButton>
        <DisclosureWithHelp
            label={t('showcase.basics.disclosure.label')}
            {lang}
            help={t('showcase.basics.disclosure.help')}
            helpTestId="showcase-basics-disclosure-help"
            openerTestId="showcase-basics-disclosure-open"
            testId="showcase-basics-disclosure"
        >
            <p>{t('showcase.basics.disclosure.body')}</p>
        </DisclosureWithHelp>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-icons">
        <HeadingWithHelp text={t('showcase.basics.icons.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-icon-buttons">
            <IconButton
                tooltip={t('showcase.basics.icon.edit')}
                description={t('showcase.basics.icon.editDescription')}
                ariaLabel={t('showcase.basics.icon.edit')}
                data-test="showcase-basics-icon-default"
            >
                {#snippet icon()}<PencilSimpleIcon aria-hidden="true" />{/snippet}
            </IconButton>
            <IconButton
                variant="delete"
                tooltip={t('showcase.basics.icon.delete')}
                ariaLabel={t('showcase.basics.icon.delete')}
                data-test="showcase-basics-icon-delete"
            >
                {#snippet icon()}<TrashIcon aria-hidden="true" />{/snippet}
            </IconButton>
            <IconButton
                variant="archive"
                tooltip={t('showcase.basics.icon.archive')}
                ariaLabel={t('showcase.basics.icon.archive')}
                data-test="showcase-basics-icon-archive"
            >
                {#snippet icon()}<ArchiveIcon aria-hidden="true" />{/snippet}
            </IconButton>
            <IconButton
                variant="unarchive"
                tooltip={t('showcase.basics.icon.restore')}
                ariaLabel={t('showcase.basics.icon.restore')}
                data-test="showcase-basics-icon-unarchive"
            >
                {#snippet icon()}<ArchiveIcon aria-hidden="true" />{/snippet}
            </IconButton>
            <IconButton
                disabled
                tooltip={t('showcase.basics.icon.disabled')}
                ariaLabel={t('showcase.basics.icon.disabled')}
                data-test="showcase-basics-icon-disabled"
            >
                {#snippet icon()}<PencilSimpleIcon aria-hidden="true" />{/snippet}
            </IconButton>
            <IconTooltip
                name={t('showcase.basics.icon.link')}
                description={t('showcase.basics.icon.linkDescription')}
                testId="showcase-basics-icon-tooltip"
            >
                <a
                    href="/showcase"
                    class="inline-flex min-h-11 min-w-11 items-center justify-center text-brand-700"
                    aria-label={t('showcase.basics.icon.link')}
                    data-test="showcase-basics-icon-tooltip-link"
                >
                    <CheckIcon aria-hidden="true" />
                </a>
            </IconTooltip>
            <AttentionIcon
                tooltip={t('showcase.basics.attention.single')}
                ariaLabel={t('showcase.basics.attention.label')}
                testId="showcase-basics-attention-one"
            />
            <AttentionIcon
                issues={[t('showcase.basics.attention.first'), t('showcase.basics.attention.second')]}
                ariaLabel={t('showcase.basics.attention.label')}
                testId="showcase-basics-attention-many"
            />
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-help">
        <HeadingWithHelp text={t('showcase.basics.help.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-4" data-test="showcase-basics-help-row">
            <span>
                {t('showcase.basics.help.word')}
                <HelpTooltip
                    tooltip={t('showcase.basics.help.text')}
                    ariaLabel={t('showcase.basics.help.word')}
                    testId="showcase-basics-help-plain"
                />
            </span>
            <span>
                {t('showcase.basics.help.withLinks')}
                <HelpTooltip
                    tooltip={t('showcase.basics.help.text')}
                    links={HELP_LINKS}
                    ariaLabel={t('showcase.basics.help.withLinks')}
                    testId="showcase-basics-help-links"
                />
            </span>
            <span>
                {t('showcase.basics.help.ownGlyph')}
                <HelpTooltip
                    tooltip={t('showcase.basics.help.text')}
                    ariaLabel={t('showcase.basics.help.ownGlyph')}
                    testId="showcase-basics-help-glyph"
                    class="text-ok-600"
                >
                    {#snippet icon()}<CheckIcon aria-hidden="true" />{/snippet}
                </HelpTooltip>
            </span>
        </div>
        <div class="flex flex-wrap gap-4" data-test="showcase-basics-link-lists">
            <HelpLinkList links={HELP_LINKS} testId="showcase-basics-links-white" tone="onWhite" />
            <div class="rounded bg-warn-600 p-2">
                <HelpLinkList links={HELP_LINKS} testId="showcase-basics-links-amber" tone="onAmber" />
            </div>
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-notes">
        <HeadingWithHelp text={t('showcase.basics.notes.title')} look="section" level="h2" />
        <NoteBox tone="warning" dataTest="showcase-basics-note-warning">{t('showcase.basics.notes.warning')}</NoteBox>
        <NoteBox tone="error" dataTest="showcase-basics-note-error">{t('showcase.basics.notes.error')}</NoteBox>
        <NoteBox tone="success" dataTest="showcase-basics-note-success">{t('showcase.basics.notes.success')}</NoteBox>
        <NoteBox tone="info" dataTest="showcase-basics-note-info">{t('showcase.basics.notes.info')}</NoteBox>
        <NoteBox tone="neutral" size="xs" dataTest="showcase-basics-note-neutral">
            {t('showcase.basics.notes.neutral')}
        </NoteBox>
        <WarningBanner
            title={t('showcase.basics.banner.title')}
            message={t('showcase.basics.banner.message')}
            actionLabel={t('showcase.basics.banner.action')}
            actionHref="/showcase"
            dataTest="showcase-basics-banner-warning"
        />
        <WarningBanner
            variant="danger"
            title={t('showcase.basics.banner.dangerTitle')}
            message={t('showcase.basics.banner.message')}
            actionLabel={t('showcase.basics.banner.action')}
            onAction={() => toast.warning(t('showcase.basics.banner.done'))}
            dataTest="showcase-basics-banner-danger"
        />
        <EmptyState message={t('showcase.basics.empty.box')} dataTest="showcase-basics-empty-box">
            <Button href="/showcase" variant="secondary">{t('showcase.basics.empty.action')}</Button>
        </EmptyState>
        <EmptyState
            message={t('showcase.basics.empty.inline')}
            variant="inline"
            dataTest="showcase-basics-empty-inline"
        />
        <EmptyState message={t('showcase.basics.empty.plain')} variant="plain" dataTest="showcase-basics-empty-plain" />
    </Stack>

    <Stack as="section" dataTest="showcase-basics-statuses">
        <HeadingWithHelp text={t('showcase.basics.statuses.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-status-badges">
            {#each TONES as { tone, label } (tone)}
                <StatusBadge {label} colorClass={toneClass(tone)} dataTest="showcase-basics-badge-{tone}" />
            {/each}
            <StatusBadge
                size="md"
                label={t('showcase.basics.tone.ok')}
                colorClass={toneClass('ok')}
                dataTest="showcase-basics-badge-md"
            />
        </div>
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-status-badges-help">
            {#each TONES as { tone, label } (tone)}
                <StatusBadgeWithHelp
                    {label}
                    {tone}
                    help={t('showcase.basics.statuses.help')}
                    size="md"
                    testId="showcase-basics-badge-help-{tone}"
                />
            {/each}
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-headings">
        <HeadingWithHelp text={t('showcase.basics.headings.title')} look="section" level="h2" />
        <PageHeading
            level="h2"
            title={t('showcase.basics.pageHeading.title')}
            help={t('showcase.basics.pageHeading.help')}
            headingTestId="showcase-basics-page-heading-title"
            helpTestId="showcase-basics-page-heading-help"
            back={{
                href: '/showcase',
                label: t('showcase.basics.pageHeading.back'),
                dataTest: 'showcase-basics-page-heading-back'
            }}
            dataTest="showcase-basics-page-heading"
        >
            {#snippet subline()}<span>{t('showcase.basics.pageHeading.subline')}</span>{/snippet}
            {#snippet actions()}
                <Button size="md" dataTest="showcase-basics-page-heading-action">
                    {t('showcase.basics.pageHeading.action')}
                </Button>
            {/snippet}
        </PageHeading>
        <HeadingWithHelp
            text={t('showcase.basics.heading.label')}
            help={t('showcase.basics.heading.help')}
            testId="showcase-basics-heading-label"
            headingTestId="showcase-basics-heading-label-text"
        />
        <HeadingWithHelp
            text={t('showcase.basics.heading.section')}
            look="section"
            level="h3"
            help={t('showcase.basics.heading.help')}
            helpLinks={HELP_LINKS}
            testId="showcase-basics-heading-section"
            headingTestId="showcase-basics-heading-section-text"
        />
        <HeadingWithHelp
            text={t('showcase.basics.heading.title')}
            look="title"
            level="h3"
            help={t('showcase.basics.heading.help')}
            testId="showcase-basics-heading-title"
            headingTestId="showcase-basics-heading-title-text"
        />
    </Stack>

    <Stack as="section" dataTest="showcase-basics-navigation">
        <HeadingWithHelp text={t('showcase.basics.navigation.title')} look="section" level="h2" />
        <BackLink href="/showcase" label={t('showcase.basics.navigation.back')} dataTest="showcase-basics-back-link" />
        <Tabs
            {tabs}
            {activeTab}
            onselect={(tabId) => (activeTab = tabId)}
            dataTest="showcase-basics-tabs"
            stripTest="showcase-basics-tabs-strip"
            ariaLabel={t('showcase.basics.tab.label')}
        />
        <StepStrip {lang} steps={STEPS} nowText={t('showcase.basics.step.now')} dataTest="showcase-basics-steps" />
        <StepStrip {lang} steps={STEPS} compact dataTest="showcase-basics-steps-compact" />
        <StepStrip {lang} steps={STEPS} look="words" dataTest="showcase-basics-steps-words" />
        <Pagination
            {lang}
            {page}
            perPage={20}
            total={137}
            onNavigate={(to) => {
                page = to;
            }}
            dataTest="showcase-basics-pagination"
        />
        <PrevNextLinks
            page={listPage}
            hasMore={listPage < 4}
            hrefFor={(to) => `/showcase?page=${to}`}
            prevLabel={t('showcase.basics.navigation.prev')}
            nextLabel={t('showcase.basics.navigation.next')}
            dataTest="showcase-basics-prev-next"
        />
        <div class="flex flex-wrap gap-2">
            <Button variant="secondary" onclick={() => (listPage = Math.max(1, listPage - 1))}>
                {t('showcase.basics.navigation.prev')}
            </Button>
            <Button variant="secondary" onclick={() => (listPage = Math.min(4, listPage + 1))}>
                {t('showcase.basics.navigation.next')}
            </Button>
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-records">
        <HeadingWithHelp text={t('showcase.basics.records.title')} look="section" level="h2" />
        <ClickableCard href="/showcase" dataTest="showcase-basics-card" linkDataTest="showcase-basics-card-link">
            {#snippet title()}{t('showcase.basics.records.card')}{/snippet}
            <p class="text-sm text-ink-600">{t('showcase.basics.records.cardBody')}</p>
            <Button variant="secondary" dataTest="showcase-basics-card-button">
                {t('showcase.basics.records.cardButton')}
            </Button>
        </ClickableCard>
        <table class="w-full text-left text-sm" data-test="showcase-basics-table">
            <tbody>
                <ClickableRow dataTest="showcase-basics-row" class="border-b border-ink-200">
                    <td class="p-2">
                        <StretchedLink href="/showcase" dataTest="showcase-basics-row-link">
                            {t('showcase.basics.records.row')}
                        </StretchedLink>
                    </td>
                    <td class="p-2 text-ink-600">{t('showcase.basics.records.rowBody')}</td>
                </ClickableRow>
            </tbody>
        </table>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-filters">
        <HeadingWithHelp text={t('showcase.basics.filters.title')} look="section" level="h2" />
        <FilteredListNote
            text={t('showcase.basics.filters.note')}
            clearLabel={t('showcase.basics.filters.clear')}
            clearHref="/showcase"
            dataTest="showcase-basics-filtered-note"
            clearDataTest="showcase-basics-filtered-note-clear"
        />
        <div class="flex flex-wrap gap-2" data-test="showcase-basics-chips">
            {#if !chipRemoved}
                <RemovableChip
                    label={t('showcase.basics.filters.chip')}
                    sublabel={t('showcase.basics.filters.chipSub')}
                    ariaLabel={t('showcase.basics.filters.chipRemove')}
                    onRemove={() => (chipRemoved = true)}
                    dataTest="showcase-basics-chip-button"
                />
            {:else}
                <Button variant="secondary" onclick={() => (chipRemoved = false)}>
                    {t('showcase.basics.filters.chipBack')}
                </Button>
            {/if}
            <RemovableChip
                label={t('showcase.basics.filters.chip')}
                ariaLabel={t('showcase.basics.filters.chipRemove')}
                href="/showcase"
                dataTest="showcase-basics-chip-link"
            />
        </div>
    </Stack>

    <Stack as="section" dataTest="showcase-basics-messages">
        <HeadingWithHelp text={t('showcase.basics.messages.title')} look="section" level="h2" />
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-copy">
            <CopyButton
                text="https://example.org/shared/link"
                label={t('showcase.basics.messages.copy')}
                copiedLabel={t('showcase.basics.messages.copied')}
                failedLabel={t('showcase.basics.messages.copyFailed')}
                dataTest="showcase-basics-copy-button"
            />
            <CopyButton
                look="icon"
                text="https://example.org/shared/link"
                label={t('showcase.basics.messages.copy')}
                copiedLabel={t('showcase.basics.messages.copied')}
                failedLabel={t('showcase.basics.messages.copyFailed')}
                dataTest="showcase-basics-copy-icon"
            />
        </div>
        <div class="flex flex-wrap items-center gap-2" data-test="showcase-basics-toasts">
            <Button variant="secondary" onclick={() => toast.success(t('showcase.basics.messages.toastSuccess'))}>
                {t('showcase.basics.messages.success')}
            </Button>
            <Button variant="secondary" onclick={() => toast.error(t('showcase.basics.messages.toastError'))}>
                {t('showcase.basics.messages.error')}
            </Button>
            <Button variant="secondary" onclick={() => toast.warning(t('showcase.basics.messages.toastWarning'))}>
                {t('showcase.basics.messages.warning')}
            </Button>
            <Button variant="secondary" onclick={() => toast.info(t('showcase.basics.messages.toastInfo'))}>
                {t('showcase.basics.messages.info')}
            </Button>
            <Button variant="secondary" onclick={showPageChangeBar} dataTest="showcase-basics-page-change">
                {t('showcase.basics.messages.pageChange')}
            </Button>
        </div>
        <Toaster {lang} />
    </Stack>
</Stack>
