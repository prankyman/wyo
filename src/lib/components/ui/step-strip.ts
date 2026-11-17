/** Where a step of a `StepStrip` stands. */
export type TStepStripState = 'done' | 'current' | 'todo';

/** One chip of a `StepStrip`: its stable key (it becomes part of the chip's `data-test`), its words and its state. */
export interface IStepStripStep {
    key: string;
    label: string;
    state: TStepStripState;
}

/**
 * The steps of a strip whose stages follow one another in a fixed order: the stages before `current` are done, `current`
 * is current and the rest are to do. When `current` is not among `keys` (or is empty) no stage is current and none is
 * done: every one is still to do.
 */
export function stageSteps<TKey extends string>(
    keys: readonly TKey[],
    current: TKey | null | undefined,
    labelOf: (key: TKey) => string
): IStepStripStep[] {
    const at = current ? keys.indexOf(current) : -1;
    return keys.map((key, index): IStepStripStep => {
        let state: TStepStripState = 'todo';
        if (at >= 0) state = index < at ? 'done' : index === at ? 'current' : 'todo';
        return { key, label: labelOf(key), state };
    });
}
