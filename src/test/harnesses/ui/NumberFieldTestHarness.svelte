<script lang="ts">
    import Field from '$lib/components/ui/Field.svelte';
    import NumberField from '$lib/components/ui/NumberField.svelte';
    import { readNumberText } from '$lib/validation/number-input';

    /** One number field inside its `Field`, read on leaving the box the way a form reads it. */
    interface IProps {
        decimals?: number;
        signed?: boolean;
        min?: number;
        max?: number;
        value?: string;
    }

    let { decimals = 0, signed = false, min, max, value = $bindable('') }: IProps = $props();
    let error = $state<string | null>(null);

    function check(): void {
        const reading = readNumberText(value, { decimals, signed, min, max });
        error = reading.ok ? null : reading.reason;
    }
</script>

<Field label="Shift" for="shift" testId="shift" dataTest="shift-field" {error}>
    <NumberField
        id="shift"
        name="shift"
        lang="ru"
        {decimals}
        {signed}
        bind:value
        dataTest="shift-input"
        onblur={check}
    />
</Field>
<output data-test="shift-value">{value}</output>
