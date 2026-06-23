-- The abuse-control check (CountInboundBySender) filters inbound_events by
-- sender + received_at on every failed inbound. Without an index on sender that
-- is a sequential scan over an ever-growing table, which an attacker's own flood
-- makes progressively more expensive (self-amplifying). Add the matching index.
CREATE INDEX IF NOT EXISTS inbound_events_sender_time ON inbound_events (sender, received_at);
