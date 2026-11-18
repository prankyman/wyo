import type { components } from '$lib/api/wyo-api';

/** One order of one account, in the connector's own states (`GET /api/v1/orders`). */
export type TOrder = components['schemas']['Order'];
export type TOrderList = components['schemas']['OrderList'];
export type TOrderState = components['schemas']['OrderState'];
export type TOrderScheme = components['schemas']['OrderScheme'];
export type TOrderEvent = components['schemas']['OrderEvent'];
/** One order with the history of its states (`GET /api/v1/orders/{id}`). */
export type TOrderDetails = components['schemas']['OrderDetails'];
/** When «Обновить» was last pressed and when it may be pressed again (`GET`/`POST /api/v1/sync/refresh`). */
export type TRefreshState = components['schemas']['RefreshState'];
