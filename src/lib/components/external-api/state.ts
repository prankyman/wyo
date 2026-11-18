import type { TTone } from '$lib/components/ui/status-tones';
import type { TExternalApiState } from './types';

/**
 * The tone of each state of «Внешний API» (`api/wyo.yaml`, `ExternalApiState`): what the owner should make of it. Not filled is
 * quiet; working is done; a key the fulfilment centre refuses, a key that may only read, an interface switched off and a refused
 * access each stop the exchange until the owner acts, so they are red; a system that does not answer or answers unreadably may
 * come back by itself, so it needs a look and no more. One place, so the connection's result and the card of «Обзор» agree.
 */
export const EXTERNAL_API_TONE: Readonly<Record<TExternalApiState, TTone>> = {
    empty: 'off',
    working: 'ok',
    key_refused: 'stop',
    read_only_key: 'stop',
    api_off: 'stop',
    forbidden: 'stop',
    unavailable: 'warn',
    no_connection: 'warn',
    unreadable: 'warn'
};
