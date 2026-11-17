/**
 * What a bottom sheet does when the finger leaves its grab handle (the handle works as a handle does). As on Android's
 * Material bottom sheet, the handle closes the sheet by a drag down or by a tap
 * (https://developer.android.com/reference/com/google/android/material/bottomsheet/BottomSheetDragHandleView); a short drag
 * that was not a flick springs back.
 */

/** A finger that moved less than this, in px, tapped rather than dragged. */
export const TAP_SLOP_PX = 6;
/** A tap is quick; a longer press that did not move closes nothing. */
const TAP_MAX_MS = 500;
/** A drag past this share of the sheet's height closes it. */
const CLOSE_SHARE = 0.25;
/** A flick down at this speed, in px per ms, closes it from any distance past `FLICK_MIN_PX`. */
const FLICK_SPEED = 0.5;
const FLICK_MIN_PX = 24;

export interface ISheetDrag {
    /** How far down the finger went, in px; a drag up counts as 0. */
    distance: number;
    /** From the finger's touch to its lift, in ms. */
    durationMs: number;
    /** The sheet's drawn height, in px. */
    sheetHeight: number;
}

/** Whether the sheet closes (`close`) or springs back (`stay`). */
export function sheetDragOutcome({ distance, durationMs, sheetHeight }: ISheetDrag): 'close' | 'stay' {
    if (distance < TAP_SLOP_PX) return durationMs <= TAP_MAX_MS ? 'close' : 'stay';
    if (distance >= sheetHeight * CLOSE_SHARE) return 'close';
    if (distance >= FLICK_MIN_PX && distance / Math.max(1, durationMs) >= FLICK_SPEED) return 'close';
    return 'stay';
}
