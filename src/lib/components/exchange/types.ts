import type { components } from '$lib/api/wyo-api';

/** The exchange's own state of an order, shown beside the marketplace's state (`Order.exchange`). */
export type TOrderExchange = components['schemas']['OrderExchange'];
/** The exchange of one order in full (`OrderDetails.exchange`). */
export type TOrderExchangeDetails = components['schemas']['OrderExchangeDetails'];
export type TExchangeState = components['schemas']['ExchangeState'];
export type TExchangeReason = components['schemas']['ExchangeReason'];
export type TAttention = components['schemas']['Attention'];
/** A problem in the wyo server's code, with the marketplace's or the fulfilment centre's own words as received. */
export type TProblem = components['schemas']['Problem'];
export type THeldReason = components['schemas']['HeldReason'];
export type TCodeVerdict = components['schemas']['CodeVerdict'];
export type TExchangeEvent = components['schemas']['ExchangeEvent'];
/** One parcel of an order at the fulfilment centre, with the marketplace's number of the part it became and its label's state. */
export type TOrderParcel = components['schemas']['OrderParcel'];
