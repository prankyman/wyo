-- When the packing report was last sent to the marketplace, written BEFORE it leaves (pack_reported_at is written only when its
-- answer comes back). An order with pack_sent_at and no pack_reported_at had a report sent whose answer the wyo server never
-- recorded: a lost answer, or a stop of the wyo server between the write and its record. The marketplace may have taken that
-- report and split the order, so the order shown split is then never taken for a cancellation.
ALTER TABLE exchange_orders ADD COLUMN pack_sent_at INTEGER;
