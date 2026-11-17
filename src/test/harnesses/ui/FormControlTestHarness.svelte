<script lang="ts">
    import SelectBox from '$lib/components/ui/SelectBox.svelte';
    import Textarea from '$lib/components/ui/Textarea.svelte';
    import TextBox from '$lib/components/ui/TextBox.svelte';
    import type { ISelectOption } from '$lib/components/ui/types';

    /**
     * Binds the value of one of the three plain controls (`TextBox`, `SelectBox`, `Textarea` without a superform) and
     * prints it, because a `$bindable` prop does not reach a test's own scope (testing-library issue #394).
     */
    interface IProps {
        control: 'text' | 'select' | 'textarea';
        initial?: string;
        options?: ISelectOption[];
        blank?: string;
        withChildren?: boolean;
        invalid?: boolean;
        size?: 'md' | 'sm';
        class?: string;
        onFocus?: (element: HTMLElement | null) => void;
    }

    let {
        control,
        initial = '',
        options,
        blank,
        withChildren = false,
        invalid = false,
        size = 'md',
        class: className = '',
        onFocus
    }: IProps = $props();

    // svelte-ignore state_referenced_locally
    let value = $state<string | null | undefined>(initial);
    let input = $state<HTMLInputElement | null>(null);
    let select = $state<HTMLSelectElement | null>(null);
    let area = $state<HTMLTextAreaElement | null>(null);
</script>

{#if control === 'text'}
    <TextBox
        bind:value
        bind:element={input}
        name="city"
        placeholder="Город"
        {invalid}
        {size}
        class={className}
        dataTest="box"
    />
{:else if control === 'select'}
    <SelectBox
        bind:value
        bind:element={select}
        name="kind"
        {options}
        {blank}
        {invalid}
        {size}
        class={className}
        data-choices="fixed"
        dataTest="box"
    >
        {#if withChildren}
            <option value="own">Своя</option>
        {/if}
    </SelectBox>
{:else}
    <Textarea bind:value bind:element={area} name="note" rows={4} {invalid} class={className} dataTest="box" />
{/if}
<output data-test="bound">{value ?? 'nothing'}</output>
<button type="button" data-test="focus" onclick={() => onFocus?.(input ?? select ?? area)}>focus</button>
