<script lang="ts">
    import Stack from '$lib/components/ui/Stack.svelte';
    import type { Snippet } from 'svelte';

    /**
     * The frame of the pages a person sees before signing in: the program's name over one column of blocks, 16 pixels apart. It
     * fills a phone's width with a gutter of 24 pixels and keeps a computer's column narrow, because a sign-in form has no use for
     * more room.
     */
    interface IProps {
        children: Snippet;
    }

    let { children }: IProps = $props();
</script>

<Stack class="min-h-screen items-center justify-center bg-ink-50 px-6 py-6">
    <main class="w-full max-w-sm">
        <p
            class="border-b border-ink-200 pb-4 text-center text-3xl font-semibold text-brand-700"
            data-test="auth-brand"
        >
            wyo
        </p>
        <Stack class="pt-4">
            {@render children()}
        </Stack>
    </main>
</Stack>
