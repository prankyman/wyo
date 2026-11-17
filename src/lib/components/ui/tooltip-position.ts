/**
 * Where a popup stands beside the control it belongs to — the «?» of `HelpTooltip` and the header icons' `IconTooltip`
 *. One set of rules for both: the popup keeps 4 px from its control and 8 px from the screen's edges, and
 * is pushed inside the screen when it does not fit where it is anchored.
 */

/** Distance kept between the popup and the viewport edge once it has to be pushed inwards. */
export const VIEWPORT_MARGIN = 8;
/** Distance kept between the popup and the control it belongs to. */
export const TRIGGER_GAP = 4;

/** Which side of its control a popup prefers; it takes the other side when it does not fit. */
export type TTooltipSide = 'above' | 'below';
/** How a popup lines up with its control: centred on it, or starting at its left or its right edge. */
export type TTooltipAlign = 'left' | 'center' | 'right';

interface IViewport {
    width: number;
    height: number;
}

function belowPlacement(trigger: DOMRect, height: number, viewport: IViewport): string {
    const topWhenBelow = trigger.bottom + TRIGGER_GAP;
    const lowestTop = Math.max(VIEWPORT_MARGIN, viewport.height - VIEWPORT_MARGIN - height);
    return `top: ${Math.min(Math.max(topWhenBelow, VIEWPORT_MARGIN), lowestTop)}px;`;
}

/**
 * Above the control when the popup prefers it and fits there; below it otherwise, pushed down from the top edge and up
 * from the bottom edge when it fits in neither. A popup preferring the space below goes above only when it does not fit
 * below and does fit above (a header icon at the top of the screen always opens downwards).
 */
export function verticalPlacement(
    trigger: DOMRect,
    height: number,
    side: TTooltipSide = 'above',
    viewport: IViewport = { width: window.innerWidth, height: window.innerHeight }
): string {
    const fitsAbove = trigger.top - TRIGGER_GAP - height >= 0;
    const fitsBelow = trigger.bottom + TRIGGER_GAP + height <= viewport.height;
    const above = side === 'above' ? fitsAbove : fitsAbove && !fitsBelow;
    if (above) return `bottom: ${viewport.height - trigger.top + TRIGGER_GAP}px;`;
    return belowPlacement(trigger, height, viewport);
}

/**
 * Anchored to the control whenever the whole popup stays on screen; otherwise pushed inside the viewport, which drops the
 * anchoring because the popup can no longer sit where the control is.
 */
export function horizontalPlacement(
    trigger: DOMRect,
    width: number,
    align: TTooltipAlign = 'center',
    viewport: IViewport = { width: window.innerWidth, height: window.innerHeight }
): string {
    const anchoredLeft =
        align === 'center'
            ? trigger.left + trigger.width / 2 - width / 2
            : align === 'right'
              ? trigger.left
              : trigger.right - width;

    if (anchoredLeft >= 0 && anchoredLeft + width <= viewport.width) {
        if (align === 'center') return `left: ${trigger.left + trigger.width / 2}px; transform: translateX(-50%);`;
        if (align === 'right') return `left: ${trigger.left}px;`;
        return `right: ${viewport.width - trigger.right}px;`;
    }

    const furthestLeft = Math.max(VIEWPORT_MARGIN, viewport.width - VIEWPORT_MARGIN - width);
    return `left: ${Math.min(Math.max(anchoredLeft, VIEWPORT_MARGIN), furthestLeft)}px;`;
}
