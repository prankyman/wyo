<script lang="ts">
    import { api } from '$lib/api/api';
    import Field from '$lib/components/ui/Field.svelte';
    import NumberField from '$lib/components/ui/NumberField.svelte';
    import SectionForm from '$lib/components/ui/SectionForm.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import { spaForm } from '$lib/forms/super-form';
    import type { ISectionSuperForm } from '$lib/forms/section-form';
    import { tError } from '$lib/i18n/form-error';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';
    import {
        fieldNameOf,
        settingBounds,
        settingsFormValues,
        settingsGroupSchema,
        refusalOfGroup,
        settingsPayload,
        type TSettingsFormValues,
        type TSettingsGroup
    } from '$lib/validation/settings-group';
    import type { Snippet } from 'svelte';

    /**
     * One group of the settings registry drawn as a folded `SectionForm` and saved with `PUT /api/v1/settings/{group}`. The
     * registry (`GET /api/v1/settings`) decides which settings exist, their kinds, bounds and defaults, so a setting a later plan
     * adds appears here with no change in this component: a switch for a `boolean`, a number box for an `integer`. The texts are
     * `settings.<key>.label` and `.help` for each setting, `settings.group.<id>.title` and `.help` for the section; a number's help
     * may name its `{min}` and `{max}`. The form is checked in the browser by the bounds the wyo server sent and checked again
     * by the wyo server, which is the authority.
     */
    interface IProps {
        lang: TLocale;
        group: TSettingsGroup;
        /** Told the group as the wyo server stored it, after a save. */
        onSaved?: (group: TSettingsGroup) => void;
        /** What belongs to the group but is not a setting (the version and the buttons of «Обновления»), under the boxes. */
        extra?: Snippet;
    }

    let { lang, group, onSaved, extra }: IProps = $props();

    // The form is made once, from the group as it was when the section was drawn: the person's typing is never replaced by a
    // later answer.
    // svelte-ignore state_referenced_locally
    const sectionGroup = group;
    const id = `settings-${sectionGroup.id}`;

    const made = spaForm(settingsGroupSchema(sectionGroup), settingsFormValues(sectionGroup), {
        id: `${id}-form`,
        resetForm: false,
        saved: 'section',
        send: async (values) => {
            const result = await api.put<TSettingsGroup>(
                `/api/v1/settings/${sectionGroup.id}`,
                settingsPayload(sectionGroup, values as TSettingsFormValues)
            );
            if (result.ok) {
                onSaved?.(result.data);
                return result;
            }
            return { ...result, refusal: refusalOfGroup(sectionGroup, result.refusal) };
        }
    });
    const { form, errors } = made;
    const section = made as unknown as ISectionSuperForm;

    const textOf = (key: string, suffix: 'label' | 'help', params?: Record<string, number>) =>
        t(`settings.${key}.${suffix}`, lang, params);
</script>

<SectionForm
    {lang}
    {id}
    title={t(`settings.group.${sectionGroup.id}.title`, lang)}
    help={t(`settings.group.${sectionGroup.id}.help`, lang)}
    form={section}
>
    <Stack>
        {#each sectionGroup.settings as setting (setting.key)}
            {@const name = fieldNameOf(sectionGroup, setting.key)}
            {@const bounds = settingBounds(setting)}
            {#if setting.kind === 'boolean'}
                <Field
                    layout="inline"
                    for="setting-{name}-box"
                    label={textOf(setting.key, 'label')}
                    help={textOf(setting.key, 'help')}
                    testId="setting-{name}"
                    dataTest="setting-{name}-field"
                    error={tError($errors[name], lang)}
                >
                    <input
                        id="setting-{name}-box"
                        type="checkbox"
                        role="switch"
                        {name}
                        checked={$form[name] === true}
                        onchange={(event) => ($form[name] = event.currentTarget.checked)}
                        class="size-5 shrink-0"
                        data-test="setting-{name}"
                    />
                </Field>
            {:else}
                <Field
                    label={textOf(setting.key, 'label')}
                    help={textOf(setting.key, 'help', { min: bounds?.min ?? 0, max: bounds?.max ?? 0 })}
                    for="setting-{name}-box"
                    testId="setting-{name}"
                    dataTest="setting-{name}-field"
                    error={tError($errors[name], lang)}
                >
                    <NumberField
                        id="setting-{name}-box"
                        {name}
                        {lang}
                        decimals={0}
                        value={String($form[name] ?? '')}
                        oninput={(event) => ($form[name] = event.currentTarget.value)}
                        class="sm:max-w-xs"
                        dataTest="setting-{name}"
                    />
                </Field>
            {/if}
        {/each}
        {#if extra}{@render extra()}{/if}
    </Stack>
</SectionForm>
