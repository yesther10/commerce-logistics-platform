CREATE TABLE IF NOT EXISTS shipments (
    shipment_id UUID PRIMARY KEY,
    order_id TEXT NOT NULL CHECK (char_length(btrim(order_id)) BETWEEN 1 AND 255),
    status TEXT NOT NULL CHECK (char_length(btrim(status)) > 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
