<script lang="ts">
    import { cn } from '$lib/utils/cn';
    import { FOCUS_RING } from './press';
    import type { IHelpLink } from './types';

    /**
     * A list of links to articles that explain a notion further: under the paragraphs of a «?» and under a
     * glossary entry. Each link opens in a new tab, so the screen the person was reading stays where it was. Nothing is
     * drawn for an empty list. Each link stands on a line of its own, not inside a sentence, so it is a whole tap target:
     * at least 44 px tall (the phone sweep's `tap-size` rule, `e2e/helpers/phone.ts`).
     */
    interface IProps {
        links: readonly IHelpLink[];
        /** `data-test` of the list; each link is `<testId>-link` unless `linkTestId` names it. */
        testId: string;
        linkTestId?: string;
        /** The links' colour: white on the amber «?» popup, amber on a white card. */
        tone?: 'onAmber' | 'onWhite';
        class?: string;
    }

    let { links, testId, linkTestId, tone = 'onWhite', class: className }: IProps = $props();

    const TONE_CLASS: Record<NonNullable<IProps['tone']>, string> = {
        onAmber: 'text-white',
        onWhite: 'text-warn-700 pressable:hover:text-warn-900'
    };
</script>

{#if links.length > 0}
    <ul class={cn('flex list-none flex-col p-0', className)} data-test={testId}>
        {#each links as link (link.href)}
            <li>
                <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex min-h-11 items-center underline {TONE_CLASS[tone]} {FOCUS_RING}"
                    data-test={linkTestId ?? `${testId}-link`}>{link.label}</a
                >
            </li>
        {/each}
    </ul>
{/if}
