<script lang="ts">
    import Field from './Field.svelte';
    import TextBox from './TextBox.svelte';

    /**
     * The «from – to» pair of dates of a filter or a report: two `Field`s, each a date `TextBox`. They are
     * drawn side by side in the caller's row or grid (no wrapper of their own), so the period sits among the other filters.
     */
    interface IProps {
        /** The first day, `YYYY-MM-DD`; bind it to read it. */
        from?: string | null;
        /** The last day, `YYYY-MM-DD`; bind it to read it. */
        to?: string | null;
        fromName?: string;
        toName?: string;
        fromLabel: string;
        toLabel: string;
        fromHelp?: string;
        toHelp?: string;
        required?: boolean;
        /** The earliest and latest day a box offers. */
        min?: string;
        max?: string;
        /** The boxes are `<testPrefix>-from` and `<testPrefix>-to`. */
        testPrefix: string;
        /** The refusals of the two days, already worded. */
        errors?: { from?: string | null; to?: string | null };
        /** Classes of each `Field` (its place in a grid). */
        fieldClass?: string;
        /** Classes merged over each box's look (its width). */
        class?: string;
        disabled?: boolean;
    }

    let {
        from = $bindable(),
        to = $bindable(),
        fromName = 'from',
        toName = 'to',
        fromLabel,
        toLabel,
        fromHelp,
        toHelp,
        required = false,
        min,
        max,
        testPrefix,
        errors = {},
        fieldClass = '',
        class: className = '',
        disabled = false
    }: IProps = $props();
</script>

<Field label={fromLabel} help={fromHelp} error={errors.from} for="{testPrefix}-from" class={fieldClass} {disabled}>
    <TextBox
        type="date"
        id="{testPrefix}-from"
        name={fromName}
        bind:value={from}
        {required}
        {min}
        {max}
        {disabled}
        invalid={Boolean(errors.from)}
        class={className}
        dataTest="{testPrefix}-from"
    />
</Field>
<Field label={toLabel} help={toHelp} error={errors.to} for="{testPrefix}-to" class={fieldClass} {disabled}>
    <TextBox
        type="date"
        id="{testPrefix}-to"
        name={toName}
        bind:value={to}
        {required}
        {min}
        {max}
        {disabled}
        invalid={Boolean(errors.to)}
        class={className}
        dataTest="{testPrefix}-to"
    />
</Field>
