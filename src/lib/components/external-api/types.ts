import type { components } from '$lib/api/wyo-api';

/** The setting «Внешний API» as the wyo server answers it (`GET /api/v1/external-api`): never the key, only its last four characters. */
export type TExternalApi = components['schemas']['ExternalApi'];
export type TExternalApiState = components['schemas']['ExternalApiState'];
/** A marketplace as the fulfilment centre's own list holds it (its own code and name). */
export type TFcMarketplace = components['schemas']['FcMarketplace'];
/** A warehouse of the fulfilment centre. */
export type TFcWarehouse = components['schemas']['FcWarehouse'];
/** The seller's warehouses beside the fulfilment centre's warehouses that stand for them (`GET /api/v1/external-api/warehouses`). */
export type TWarehouseMatches = components['schemas']['WarehouseMatches'];
export type TWarehouseMatch = components['schemas']['WarehouseMatch'];
/** Each account beside the marketplace's code in the fulfilment centre's list (`GET /api/v1/external-api/marketplaces`). */
export type TMarketplaceMatches = components['schemas']['MarketplaceMatches'];
export type TMarketplaceMatch = components['schemas']['MarketplaceMatch'];
/** How the wyo server learns what the fulfilment centre did (`GET /api/v1/external-api/events`). */
export type TEventsSetting = components['schemas']['EventsSetting'];
export type TEventsMode = components['schemas']['EventsMode'];
/** One hand-over setting an adapter declares (`Marketplace.handoverSettings`). */
export type TSettingField = components['schemas']['SettingField'];
