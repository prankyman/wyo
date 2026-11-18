import { api } from '$lib/api/api';
import type { THandoverDetails, THandoverList } from '$lib/components/handovers/types';
import type { PageLoad } from './$types';

/**
 * «Передачи» reads what its address names: the list of hand-overs (`GET /api/v1/handovers`) and the hand-over whose details are
 * open beside the list (`?handover=<id>`, `GET /api/v1/handovers/{id}`). A list that cannot be read leaves its refusal's code for
 * the page to word.
 */
export const load: PageLoad = async ({ url }) => {
    const opened = url.searchParams.get('handover');
    const [list, details] = await Promise.all([
        api.get<THandoverList>('/api/v1/handovers'),
        opened && /^\d+$/.test(opened) ? api.get<THandoverDetails>(`/api/v1/handovers/${opened}`) : null
    ]);
    return {
        list: list.ok ? list.data : null,
        refusal: list.ok ? null : list.refusal.code,
        opened: details?.ok ? details.data : null
    };
};
