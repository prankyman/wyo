<script lang="ts">
    import DateRangeFields from '$lib/components/ui/DateRangeFields.svelte';

    /** Binds `DateRangeFields`' two days and prints them (a `$bindable` prop does not reach a test's own scope). */
    interface IProps {
        initialFrom?: string;
        initialTo?: string;
        required?: boolean;
        min?: string;
        max?: string;
        errors?: { from?: string | null; to?: string | null };
        fromName?: string;
        toName?: string;
    }

    let { initialFrom = '', initialTo = '', required, min, max, errors, fromName, toName }: IProps = $props();

    // svelte-ignore state_referenced_locally
    let from = $state<string | null | undefined>(initialFrom);
    // svelte-ignore state_referenced_locally
    let to = $state<string | null | undefined>(initialTo);
</script>

<div data-test="row">
    <DateRangeFields
        bind:from
        bind:to
        fromLabel="С"
        toLabel="По"
        fromHelp="Первый день"
        {fromName}
        {toName}
        {required}
        {min}
        {max}
        {errors}
        testPrefix="period"
        fieldClass="w-40"
    />
</div>
<output data-test="bound">{from}…{to}</output>
