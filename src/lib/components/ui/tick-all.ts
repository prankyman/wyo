/** What the tick box in a list's heading shows (`TickAllBox`): no row ticked, some of them, or every one. */
export type TTickAllState = 'none' | 'some' | 'all';

/** The heading box's state for `ticked` rows of the `total` that can be ticked. */
export function tickAllState(ticked: number, total: number): TTickAllState {
    if (ticked === 0) return 'none';
    return ticked >= total ? 'all' : 'some';
}
