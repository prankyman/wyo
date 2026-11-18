import type { components } from '$lib/api/wyo-api';

/** One hand-over: the parcels of one day that leave the fulfilment centre together for the marketplace. */
export type THandover = components['schemas']['Handover'];
export type THandoverList = components['schemas']['HandoverList'];
export type THandoverState = components['schemas']['HandoverState'];
export type THandoverStep = components['schemas']['HandoverStep'];
export type THandoverStepState = components['schemas']['HandoverStepState'];
/** A hand-over with the strip of its steps and what the owner can do (`GET /api/v1/handovers/{id}`). */
export type THandoverDetails = components['schemas']['HandoverDetails'];
export type TShippingPoint = components['schemas']['ShippingPoint'];
export type TShippingPointChoice = components['schemas']['ShippingPointChoice'];
