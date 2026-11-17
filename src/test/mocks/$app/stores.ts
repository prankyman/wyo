import { readable, readonly, writable } from 'svelte/store';

export interface IPageState {
    url: URL;
    params: Record<string, string>;
    route: { id: string | null };
    status?: number;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    form?: any;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    data?: any;
    error?: Error | null;
}

const initialPageState: IPageState = {
    url: new URL('http://localhost/'),
    params: {},
    route: { id: null },
    status: 200,
    form: undefined,
    data: undefined,
    error: null
};

const pageWritable = writable<IPageState>({ ...initialPageState });

export const page = readonly(pageWritable);
export const navigating = readable<unknown>(null);

export function setPageState(state: Partial<IPageState>): void {
    pageWritable.update((current) => ({ ...current, ...state }));
}

export function resetPageState(): void {
    pageWritable.set({ ...initialPageState });
}
