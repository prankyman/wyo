<script lang="ts">
    interface IProps {
        label: string;
        colorClass: string;
        size?: 'sm' | 'md';
        class?: string;
        dataTest?: string;
        title?: string;
    }

    let { label, colorClass, size = 'sm', class: className = '', dataTest, title }: IProps = $props();

    const sizeClasses = $derived(size === 'md' ? 'px-3 py-1 text-xs' : 'px-2 py-0.5 text-[10px]');
</script>

<span class="inline-block font-medium {sizeClasses} {colorClass} {className}" data-test={dataTest} {title}>{label}</span
>
