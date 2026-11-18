<script lang="ts">
    import type { TMarketplace } from '$lib/components/marketplaces/types';
    import Field from '$lib/components/ui/Field.svelte';
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Stack from '$lib/components/ui/Stack.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import { t } from '$lib/i18n/i18n';
    import type { TLocale } from '$lib/i18n/types';

    /**
     * The boxes of one account's hand-over settings, drawn only from what its adapter declares (`Marketplace.handoverSettings`),
     * never from a marketplace's name: a drop-down for a setting that has choices, a text box for one that has none. The texts of a
     * setting are `marketplace.<code>.handover.<name>.label`, `.help` and, for each choice, `.choice.<value>`, so a marketplace
     * that declares a setting brings its own words with it. The values are the parent form's (`values`, by the setting's name; a
     * change is told to `onChange`); a refusal is laid on its setting (`errors`, already worded).
     */
    interface IProps {
        lang: TLocale;
        marketplace: TMarketplace;
        /** The value of each setting by its name; an unset one is missing or empty. */
        values: Record<string, string>;
        /** The owner chose or typed `value` for the setting `name`. */
        onChange: (name: string, value: string) => void;
        errors?: Record<string, string | undefined>;
        /** A setting's field is `<idPrefix>-<name>-field`, its box `<idPrefix>-<name>-box`. */
        idPrefix: string;
    }

    let { lang, marketplace, values, onChange, errors = {}, idPrefix }: IProps = $props();

    const text = (name: string, part: string) => t(`marketplace.${marketplace.code}.handover.${name}.${part}`, lang);
</script>

<Stack dataTest={idPrefix}>
    {#each marketplace.handoverSettings as setting (setting.name)}
        <Field
            label={text(setting.name, 'label')}
            help={text(setting.name, 'help')}
            for="{idPrefix}-{setting.name}-box"
            testId="{idPrefix}-{setting.name}"
            dataTest="{idPrefix}-{setting.name}-field"
            error={errors[setting.name]}
        >
            {#if setting.choices.length > 0}
                <SelectBox
                    id="{idPrefix}-{setting.name}-box"
                    name={setting.name}
                    value={values[setting.name] ?? ''}
                    onchange={(event) => onChange(setting.name, event.currentTarget.value)}
                    blank={t('externalApi.handover.blank', lang)}
                    options={setting.choices.map((choice) => ({
                        value: choice,
                        label: text(setting.name, `choice.${choice}`)
                    }))}
                    invalid={Boolean(errors[setting.name])}
                    class="sm:max-w-md"
                    dataTest="{idPrefix}-{setting.name}-box"
                />
            {:else}
                <TextBox
                    id="{idPrefix}-{setting.name}-box"
                    name={setting.name}
                    value={values[setting.name] ?? ''}
                    oninput={(event) => onChange(setting.name, event.currentTarget.value)}
                    invalid={Boolean(errors[setting.name])}
                    class="sm:max-w-md"
                    dataTest="{idPrefix}-{setting.name}-box"
                />
            {/if}
        </Field>
    {/each}
</Stack>
