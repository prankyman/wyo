<script lang="ts">
    import type { Snippet } from 'svelte';
    import { z } from 'zod';
    import { page } from '$app/state';
    import Button from '$lib/components/ui/Button.svelte';
    import CollapsibleSection from '$lib/components/ui/CollapsibleSection.svelte';
    import ConfirmationModal from '$lib/components/ui/ConfirmationModal.svelte';
    import DisclosureForm from '$lib/components/ui/DisclosureForm.svelte';
    import Field from '$lib/components/ui/Field.svelte';
    import FieldLabel from '$lib/components/ui/FieldLabel.svelte';
    import FieldMessage from '$lib/components/ui/FieldMessage.svelte';
    import FieldRefusal from '$lib/components/ui/FieldRefusal.svelte';
    import FilterSection from '$lib/components/ui/FilterSection.svelte';
    import FormAnswer from '$lib/components/ui/FormAnswer.svelte';
    import FormButtons from '$lib/components/ui/FormButtons.svelte';
    import ModalWindow from '$lib/components/ui/ModalWindow.svelte';
    import NumberField from '$lib/components/ui/NumberField.svelte';
    import PopupCloseButton from '$lib/components/ui/PopupCloseButton.svelte';
    import RowDetailsPanel from '$lib/components/ui/RowDetailsPanel.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import ShowOnceSecret from '$lib/components/ui/ShowOnceSecret.svelte';
    import SlideOverPanel from '$lib/components/ui/SlideOverPanel.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import SubmitWithAnswer from '$lib/components/ui/SubmitWithAnswer.svelte';
    import Textarea from '$lib/components/ui/Textarea.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import WindowButtons from '$lib/components/ui/WindowButtons.svelte';
    import { sectionSchema } from '$lib/forms/section-form';
    import { spaForm } from '$lib/forms/super-form';
    import { getLocale, t } from '$lib/i18n/i18n';
    import ShowcaseRealForm from './ShowcaseRealForm.svelte';

    /**
     * The showcase of the windows, the fields and the forms of the component library: every component of that part drawn in each
     * of its states, with texts from `messages/ru.json`, so the maintainer judges the theme on one page and the browser tests
     * measure it at 390 pixels. `ShowcaseRealForm` is its one real form: a zod schema, sent through a stub (no server).
     */
    const lang = getLocale();
    const text = (key: string, params?: Record<string, string | number>): string =>
        t(`showcase.forms.${key}`, lang, params);

    const kinds = [
        { value: 'first', label: text('option.first') },
        { value: 'second', label: text('option.second') },
        { value: 'third', label: text('option.third'), disabled: true }
    ];

    // The windows: each opens from its own button, which stands outside the window.
    let windowOpen = $state(false);
    let sheetOpen = $state(false);
    let confirmOpen = $state(false);
    let panelOpen = $state(false);
    let responsiveOpen = $state(false);

    // A record in a side panel: `?edit=sample` in the address opens it, and a plain press on the link opens it without a reload.
    const RECORD = { id: 'sample', name: text('window.title') };
    const recordFromAddress = $derived(page.url.searchParams.get('edit') === RECORD.id ? RECORD : null);

    // The values of the boxes that stand alone.
    let name = $state('');
    let kind = $state('first');
    let note = $state('');
    let amount = $state('12,5');
    let shift = $state('-3');

    // A section of an edit screen is one form of its own; its save is answered by the stub with the code `section`.
    const sectionMade = spaForm(
        sectionSchema({ title: z.string().min(2, 'errors.invalid') }, ['title']),
        { title: text('section.edit'), drawn: '' },
        { id: 'showcase-section-form', resetForm: false, saved: 'section', send: async () => ({ ok: true, data: {} }) }
    );
    const { form: sectionValues } = sectionMade;
</script>

{#snippet group(id: string, title: string, body: Snippet)}
    <section class="flex flex-col gap-4 rounded border border-ink-200 bg-white p-4" data-test="showcase-forms-{id}">
        <h2 class="text-lg">{title}</h2>
        {@render body()}
    </section>
{/snippet}

{#snippet fields()}
    <Field
        label={text('label.name')}
        help={text('help.name')}
        for="showcase-field-plain"
        testId="showcase-field-plain-field"
    >
        <TextBox id="showcase-field-plain" bind:value={name} dataTest="showcase-field-plain" />
    </Field>
    <Field
        label={text('label.name')}
        for="showcase-field-refused"
        testId="showcase-field-refused-field"
        error={text('refusal')}
    >
        <TextBox id="showcase-field-refused" value="" invalid dataTest="showcase-field-refused" />
    </Field>
    <Field label={text('label.hidden')} labelHidden for="showcase-field-hidden" testId="showcase-field-hidden-field">
        <TextBox id="showcase-field-hidden" value="" dataTest="showcase-field-hidden" />
    </Field>
    <Field label={text('label.name')} disabled for="showcase-field-disabled" testId="showcase-field-disabled-field">
        <TextBox id="showcase-field-disabled" value="" disabled dataTest="showcase-field-disabled" />
    </Field>
    <Field label={text('label.aside')} for="showcase-field-aside" testId="showcase-field-aside-field">
        {#snippet aside()}<span class="ml-2 text-xs text-ink-500">{text('aside')}</span>{/snippet}
        <TextBox id="showcase-field-aside" value="" dataTest="showcase-field-aside" />
    </Field>
    <Field label={text('label.agree')} layout="inline" testId="showcase-field-inline-field">
        <input type="checkbox" class="size-5" data-test="showcase-field-inline" />
    </Field>
    <Field label={text('label.row')} layout="row" for="showcase-field-row" testId="showcase-field-row-field">
        <TextBox id="showcase-field-row" value="" dataTest="showcase-field-row" />
    </Field>
    <div class="flex flex-col gap-2">
        <FieldLabel text={text('label.name')} help={text('help.name')} testId="showcase-label" />
        <FieldMessage {lang} messages={['errors.invalid']} dataTest="showcase-message" />
        <FieldRefusal error={text('refusal')} testId="showcase-refusal">
            <TextBox value="" invalid dataTest="showcase-refusal-box" />
        </FieldRefusal>
    </div>
{/snippet}

{#snippet boxes()}
    <div class="grid gap-4 sm:grid-cols-2">
        <TextBox bind:value={name} placeholder={text('state.normal')} dataTest="showcase-text-normal" />
        <TextBox value="" size="sm" placeholder={text('state.small')} dataTest="showcase-text-small" />
        <TextBox value="" invalid placeholder={text('state.invalid')} dataTest="showcase-text-invalid" />
        <TextBox value="" disabled placeholder={text('state.disabled')} dataTest="showcase-text-disabled" />
        <SelectBox bind:value={kind} options={kinds} dataTest="showcase-select-normal" />
        <SelectBox value="" size="sm" blank={text('blank')} options={kinds} dataTest="showcase-select-small" />
        <SelectBox value="" invalid blank={text('blank')} options={kinds} dataTest="showcase-select-invalid" />
        <SelectBox value="first" disabled options={kinds} dataTest="showcase-select-disabled" />
    </div>
    <Textarea bind:value={note} {lang} testId="showcase-textarea-normal" />
    <Textarea value="" {lang} maxlength={50} invalid testId="showcase-textarea-invalid" />
    <Textarea value="" {lang} disabled testId="showcase-textarea-disabled" />
{/snippet}

{#snippet numbers()}
    <div class="grid gap-4 sm:grid-cols-2">
        <Field label={text('label.count')} for="showcase-number-whole" testId="showcase-number-whole-field">
            <NumberField id="showcase-number-whole" {lang} value="3" dataTest="showcase-number-whole" />
        </Field>
        <Field label={text('label.amount')} for="showcase-number-places" testId="showcase-number-places-field">
            <NumberField
                id="showcase-number-places"
                {lang}
                decimals={2}
                bind:value={amount}
                dataTest="showcase-number-places"
            />
        </Field>
        <Field label={text('label.shift')} for="showcase-number-signed" testId="showcase-number-signed-field">
            <NumberField
                id="showcase-number-signed"
                {lang}
                decimals={1}
                signed
                bind:value={shift}
                dataTest="showcase-number-signed"
            />
        </Field>
        <Field
            label={text('label.count')}
            disabled
            for="showcase-number-disabled"
            testId="showcase-number-disabled-field"
        >
            <NumberField id="showcase-number-disabled" {lang} value="3" disabled dataTest="showcase-number-disabled" />
        </Field>
    </div>
{/snippet}

{#snippet answers()}
    <FormAnswer saved={text('answer.saved')} dataTest="showcase-answer-saved" standalone />
    <FormAnswer error={text('answer.error')} dataTest="showcase-answer-error" standalone />
    <FormButtons dataTest="showcase-form-buttons">
        <Button>{t('app.common.save', lang)}</Button>
        <Button variant="secondary">{t('app.common.cancel', lang)}</Button>
        {#snippet below()}
            <FormAnswer saved={text('answer.saved')} dataTest="showcase-buttons-answer" />
        {/snippet}
    </FormButtons>
    <SubmitWithAnswer
        {lang}
        label={t('app.common.save', lang)}
        dataTest="showcase-submit"
        answer={{ error: 'errors.not_found' }}
    />
    <div class="relative h-14 rounded bg-ink-800 p-2" data-test="showcase-popup-close">
        <PopupCloseButton onclick={() => {}} testId="showcase-popup-close-button" />
    </div>
{/snippet}

{#snippet realForm()}
    <p class="text-sm text-ink-600">{text('form.hint')}</p>
    <ShowcaseRealForm id="real-form" />
{/snippet}

{#snippet windows()}
    <div class="flex flex-wrap gap-2">
        <Button onclick={() => (windowOpen = true)} dataTest="showcase-open-window">{text('open.window')}</Button>
        <Button onclick={() => (sheetOpen = true)} dataTest="showcase-open-sheet">{text('open.sheet')}</Button>
        <Button variant="danger" onclick={() => (confirmOpen = true)} dataTest="showcase-open-confirm">
            {text('open.confirm')}
        </Button>
        <Button variant="secondary" onclick={() => (panelOpen = true)} dataTest="showcase-open-panel">
            {text('open.panel')}
        </Button>
        <Button variant="secondary" onclick={() => (responsiveOpen = true)} dataTest="showcase-open-responsive">
            {text('open.responsive')}
        </Button>
        <Button variant="secondary" href="?edit={RECORD.id}" dataTest="showcase-open-row">{text('open.row')}</Button>
    </div>
{/snippet}

{#snippet sections()}
    <CollapsibleSection
        {lang}
        id="showcase-section-closed"
        title={text('section.title')}
        help={text('help.section')}
        headingLevel="h3"
    >
        {#snippet summary()}<span class="ml-2 text-sm text-ink-500">{text('section.summary')}</span>{/snippet}
        <p class="text-sm">{text('section.body')}</p>
    </CollapsibleSection>
    <CollapsibleSection {lang} id="showcase-section-open" title={text('section.open')} headingLevel="h3" open>
        <p class="text-sm">{text('section.body')}</p>
    </CollapsibleSection>
    <SectionForm
        {lang}
        id="showcase-section-edit"
        title={text('section.edit')}
        help={text('help.section')}
        saveHelp={text('help.save')}
        headingLevel="h3"
        form={sectionMade}
    >
        <Field label={text('label.name')} for="showcase-section-title" testId="showcase-section-title-field">
            <TextBox
                id="showcase-section-title"
                name="title"
                bind:value={$sectionValues.title}
                dataTest="showcase-section-title"
            />
        </Field>
    </SectionForm>
    <SectionForm {lang} id="showcase-section-create" title={text('section.open')} headingLevel="h3" create>
        <p class="text-sm">{text('section.body')}</p>
    </SectionForm>
    <FilterSection {lang} id="showcase-filter-none" chips={[]}>
        <TextBox value="" dataTest="showcase-filter-box" />
    </FilterSection>
    <FilterSection
        {lang}
        id="showcase-filter-used"
        resetHref="/showcase"
        chips={[
            { key: 'color', label: text('chip.color'), href: '/showcase?size=m' },
            { key: 'size', label: text('chip.size'), href: '/showcase?color=red' }
        ]}
    >
        <TextBox value="" dataTest="showcase-filter-used-box" />
    </FilterSection>
    <DisclosureForm
        {lang}
        label={text('disclosure.open')}
        testId="showcase-disclosure-edit"
        openerTestId="showcase-disclosure-edit-open"
    >
        <ShowcaseRealForm id="showcase-disclosure-edit-form" />
    </DisclosureForm>
    <DisclosureForm
        {lang}
        label={text('disclosure.add')}
        after="stay"
        testId="showcase-disclosure-add"
        openerTestId="showcase-disclosure-add-open"
    >
        <ShowcaseRealForm id="showcase-disclosure-add-form" />
    </DisclosureForm>
{/snippet}

{#snippet secret()}
    <ShowOnceSecret
        {lang}
        dataTest="showcase-once"
        title={text('secret.label')}
        help={text('secret.help')}
        note={text('secret.note')}
        items={[{ label: text('secret.label'), value: 'wyo-key-1234-abcd', testId: 'showcase-once-key' }]}
    />
{/snippet}

{#snippet dimming()}
    <div class="flex flex-wrap items-center gap-2">
        <Button disabled dataTest="showcase-button-disabled">{text('state.disabled')}</Button>
        <Button pending dataTest="showcase-button-busy">{text('state.busy')}</Button>
    </div>
{/snippet}

<Stack dataTest="showcase-forms">
    <h2 class="text-xl">{text('title')}</h2>
    {@render group('fields', text('fields.title'), fields)}
    {@render group('boxes', text('boxes.title'), boxes)}
    {@render group('numbers', text('numbers.title'), numbers)}
    {@render group('answers', text('answers.title'), answers)}
    {@render group('form', text('form.title'), realForm)}
    {@render group('windows', text('windows.title'), windows)}
    {@render group('sections', text('sections.title'), sections)}
    {@render group('secret', text('secret.title'), secret)}
    {@render group('dimming', text('dimming.title'), dimming)}
</Stack>

<ModalWindow
    bind:open={windowOpen}
    title={text('window.title')}
    dataTest="showcase-window"
    closeLabel={t('common.close', lang)}
>
    <p class="text-sm">{text('window.text')}</p>
    {#snippet footer()}
        <WindowButtons>
            <Button variant="secondary" onclick={() => (windowOpen = false)}>{t('app.common.cancel', lang)}</Button>
            <Button onclick={() => (windowOpen = false)}>{t('app.common.save', lang)}</Button>
        </WindowButtons>
    {/snippet}
</ModalWindow>

<ModalWindow bind:open={sheetOpen} title={text('window.title')} placement="sheet" dataTest="showcase-sheet">
    <p class="text-sm">{text('window.text')}</p>
    {#snippet footer()}
        <WindowButtons>
            <Button variant="secondary" onclick={() => (sheetOpen = false)}>{t('common.close', lang)}</Button>
        </WindowButtons>
    {/snippet}
</ModalWindow>

<ConfirmationModal
    bind:open={confirmOpen}
    title={text('confirm.title')}
    message={text('confirm.message')}
    confirmText={text('confirm.ok')}
    dataTest="showcase-confirm"
/>

<SlideOverPanel
    {lang}
    open={panelOpen}
    title={text('window.title')}
    onClose={() => (panelOpen = false)}
    dataTest="showcase-panel"
>
    <div class="p-4 text-sm">{text('window.text')}</div>
</SlideOverPanel>

<SlideOverPanel
    {lang}
    open={responsiveOpen}
    title={text('window.title')}
    placement="responsive"
    onClose={() => (responsiveOpen = false)}
    dataTest="showcase-responsive"
>
    <div class="p-4 text-sm">{text('window.text')}</div>
</SlideOverPanel>

<RowDetailsPanel
    {lang}
    param="edit"
    fromLoad={recordFromAddress}
    idOf={(record) => record.id}
    pick={() => RECORD}
    listHref="/showcase"
    title={(record) => text('record.title', { name: record.name })}
    dataTest="showcase-row"
>
    {#snippet children(record)}
        <div class="p-4 text-sm">{record.name}</div>
    {/snippet}
</RowDetailsPanel>
