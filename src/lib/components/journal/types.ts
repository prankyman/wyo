import type { components } from '$lib/api/wyo-api';

/** One request the wyo server sent to a marketplace (`GET /api/v1/journal`): never a key, a query or a body. */
export type TJournalEntry = components['schemas']['JournalEntry'];
export type TJournalList = components['schemas']['JournalList'];
export type TJournalOutcome = components['schemas']['JournalOutcome'];
