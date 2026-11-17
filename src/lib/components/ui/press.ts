import { cn } from '$lib/utils/cn';
import type { TButtonSizeEnum, TButtonVariantEnum } from './types';

/**
 * The pressed look every pressable control shares (constitution, press feedback
 * and single submission).
 *
 * - `touch-manipulation` drops the browser's double-tap-to-zoom wait, so the press shows at once.
 * - The transparent tap highlight removes the grey flash a phone paints over a tapped link; the
 *   control's own pressed state replaces it.
 * - `active:scale-[0.97]` is the "it went down under my finger" cue. People who asked their system
 *   for less motion keep the colour change and lose the shrink (`motion-safe:`).
 * - Every hover and pressed look — here, in `VARIANTS` and in a screen's extra classes — sits behind
 *   `pressable:` (`src/app.css`): a greyed-out (`disabled`) or busy (`aria-disabled`) control does
 *   not shrink, darken or change under the pointer. It is not going to do anything, and looking
 *   pressed would say it did. `press-feedback-policy.test.ts` fails a hover or pressed class
 *   without it.
 *
 * Use it on the few raw elements that cannot be a `Button` (the allowlist in
 * `press-feedback-policy.test.ts`); a `Button` already carries it.
 */
export const PRESS_FEEDBACK =
    'touch-manipulation [-webkit-tap-highlight-color:transparent] transition-[transform,background-color,border-color,color,opacity] duration-100 ease-out motion-safe:pressable:active:scale-[0.97] motion-reduce:transition-none';

/**
 * The tap box of a link that stands on its own (not inside a sentence): at least 44 × 44 px, the bar every tap target meets
 * (WCAG 2.5.5). A link inside a sentence keeps its inline look and is exempt, as WCAG exempts it.
 */
export const LINK_TARGET = 'inline-flex min-h-11 min-w-11 items-center';

/** A keyboard focus ring that a mouse click does not draw. */
export const FOCUS_RING = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600';

/**
 * A card that opens its record (`ClickableCard`): pressed — the stretched link under the finger is
 * `:active` — it shrinks a little, darkens and firms its border; while its record is opening it
 * dims, and neither hover nor a second press changes it (`pressable:` counts a card whose link is
 * opening as busy). `has-[…]` rather than `active:`, because `:active` also matches the ancestors of
 * a pressed checkbox or button inside the card, which open nothing.
 */
export const CLICKABLE_CARD =
    'clickable-surface clickable-card rounded border border-ink-200 bg-white px-4 py-3 touch-manipulation [-webkit-tap-highlight-color:transparent] transition-[transform,background-color,border-color,opacity] duration-100 ease-out pressable:hover:border-ink-400 motion-safe:pressable:has-[[data-stretched-link]:active]:scale-[0.99] pressable:has-[[data-stretched-link]:active]:border-ink-500 pressable:has-[[data-stretched-link]:active]:bg-ink-100 has-[[data-stretched-link][aria-busy=true]]:opacity-70 motion-reduce:transition-none';

/** A table row that opens its record (`ClickableRow`): the same states, without the shrink. */
export const CLICKABLE_ROW =
    'clickable-surface clickable-row touch-manipulation [-webkit-tap-highlight-color:transparent] transition-[background-color,opacity] duration-100 ease-out pressable:hover:bg-ink-50 pressable:has-[[data-stretched-link]:active]:bg-ink-100 has-[[data-stretched-link][aria-busy=true]]:bg-ink-100 has-[[data-stretched-link][aria-busy=true]]:opacity-70 motion-reduce:transition-none';

const BASE =
    'inline-flex items-center justify-center gap-2 rounded text-center font-medium select-none disabled:cursor-not-allowed disabled:opacity-50 aria-disabled:cursor-not-allowed aria-disabled:opacity-60 aria-busy:cursor-wait';

const SIZES: Record<TButtonSizeEnum, string> = {
    md: 'min-h-11 min-w-11 px-4 text-xs',
    lg: 'min-h-14 px-4 text-sm',
    xl: 'min-h-16 px-4 text-base'
};

/** A button that is only an icon is a square of the same height. */
const ICON_ONLY_SIZES: Record<TButtonSizeEnum, string> = {
    md: 'min-w-11 px-0',
    lg: 'min-w-14 px-0',
    xl: 'min-w-16 px-0'
};

/**
 * Whole class strings per variant, never a colour spliced into one: Tailwind generates only the
 * classes it can read in the source. Pressed is always a step darker than hover, so a mouse press
 * shows too (`active:` is emitted after `hover:` and wins). Both only on a control that can act
 * (`pressable:`); a screen's extra hover or pressed class on a `Button` carries `pressable:` as well,
 * so `cn()` sees it as the same state and lets it replace the variant's.
 */
const VARIANTS: Record<TButtonVariantEnum, string> = {
    primary: 'bg-brand-600 text-white pressable:hover:bg-brand-700 pressable:active:bg-brand-800',
    secondary:
        'border border-ink-300 bg-white text-ink-700 pressable:hover:border-ink-900 pressable:active:border-ink-900 pressable:active:bg-ink-200',
    danger: 'bg-stop-700 text-white pressable:hover:bg-stop-800 pressable:active:bg-stop-950',
    'danger-outline':
        'border border-stop-300 bg-white text-stop-800 pressable:hover:border-stop-700 pressable:active:border-stop-700 pressable:active:bg-stop-100',
    success: 'bg-ok-700 text-white pressable:hover:bg-ok-800 pressable:active:bg-ok-950',
    warning:
        'border border-warn-400 bg-transparent text-warn-900 pressable:hover:border-warn-700 pressable:active:border-warn-700 pressable:active:bg-warn-100',
    ghost: 'text-ink-600 pressable:hover:bg-ink-100 pressable:hover:text-ink-900 pressable:active:bg-ink-200 pressable:active:text-ink-900',
    'ghost-inverse': 'text-white pressable:hover:bg-white/10 pressable:active:bg-white/25 focus-visible:outline-white',
    link: 'px-1 text-sm text-brand-700 underline underline-offset-2 pressable:hover:text-brand-900 pressable:active:bg-brand-100 pressable:active:text-brand-950'
};

export interface IButtonClassOptions {
    variant: TButtonVariantEnum;
    size: TButtonSizeEnum;
    fullWidth: boolean;
    iconOnly: boolean;
    extra?: string;
}

/** The class list of a `Button`; `extra` comes last and wins any conflict through `cn()`. */
export function buttonClasses({ variant, size, fullWidth, iconOnly, extra }: IButtonClassOptions): string {
    return cn(
        BASE,
        PRESS_FEEDBACK,
        FOCUS_RING,
        SIZES[size],
        iconOnly && ICON_ONLY_SIZES[size],
        VARIANTS[variant],
        fullWidth && 'w-full',
        extra
    );
}
