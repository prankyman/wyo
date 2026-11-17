<script lang="ts">
    import { cn } from '$lib/utils/cn';
    import type { HTMLAnchorAttributes } from 'svelte/elements';
    import { FOCUS_RING, LINK_TARGET, PRESS_FEEDBACK } from './press';

    /**
     * The «← назад» link at the top of a detail screen, drawn once for every screen.
     *
     * - The look is the small grey uppercase text the screens always had; the arrow is drawn here, in front
     *   of `label`. A message that already begins with its own arrow passes `arrow={false}`.
     * - The tap box is at least 44 × 44 px in the normal layout (`LINK_TARGET`, WCAG 2.5.5). There is no
     *   negative margin: a box stretched over its neighbours would take their taps. The row around the link
     *   is 44 px tall; a screen that wants the link beside other controls puts it in a row with
     *   `items-center`.
     * - Pressed and keyboard-focus looks come from `press.ts`, like every pressable control.
     *
     * Every other link attribute (`onclick`, `aria-label`, `data-sveltekit-*`) goes straight to the `<a>`.
     */
    interface IProps extends Omit<HTMLAnchorAttributes, 'class' | 'children' | 'href'> {
        href: string;
        /** The words after the arrow, already translated. */
        label: string;
        dataTest: string;
        /** The `data-tour` anchor a guided tour points at, when the link has one. */
        dataTour?: string;
        /** Extra classes, merged with `cn()`: a conflicting utility here wins over the link's own. */
        class?: string;
        /** Draw «← » before the label. Default true. */
        arrow?: boolean;
    }

    let { href, label, dataTest, dataTour, class: className, arrow = true, ...rest }: IProps = $props();
</script>

<a
    {...rest}
    {href}
    data-test={dataTest}
    data-tour={dataTour}
    class={cn(
        LINK_TARGET,
        'text-xs tracking-wider text-ink-500 uppercase pressable:hover:text-ink-900',
        PRESS_FEEDBACK,
        FOCUS_RING,
        className
    )}
>
    {arrow ? `← ${label}` : label}
</a>
